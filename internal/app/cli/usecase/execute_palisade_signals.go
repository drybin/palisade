package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/helpers"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const (
	signalOrderQuoteUSDT   = 10.0
	signalBuyTimeout       = 10 * time.Minute
	signalRepriceDelta     = 0.002
	supportBreakPercent    = 0.003
	hardLossPercent        = 0.008
	maxPositionHold        = 120 * time.Minute
	maxEmergencySpread     = 0.01
	emergencyPriceDiscount = 0.001
)

type IExecutePalisadeSignals interface {
	Process(context.Context, bool) error
}

type ExecutePalisadeSignals struct {
	api       *webapi.MexcWebapi
	stateRepo repo.IStateRepository
	telegram  *webapi.TelegramWebapi
}

func NewExecutePalisadeSignalsUsecase(api *webapi.MexcWebapi, stateRepo repo.IStateRepository, telegram *webapi.TelegramWebapi) *ExecutePalisadeSignals {
	return &ExecutePalisadeSignals{api: api, stateRepo: stateRepo, telegram: telegram}
}

// Process executes at most one new signal per run. The live flag is deliberately
// explicit so an accidental command invocation cannot place an order.
func (u *ExecutePalisadeSignals) Process(ctx context.Context, live bool) error {
	releaseLock, acquired, err := acquireTradingLock(ctx, u.stateRepo)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer releaseLock()

	if !live {
		fmt.Println("Режим просмотра: ордера не размещаются. Для торговли нужен флаг --live")
	}

	account, err := u.api.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("get account balance: %w", err)
	}
	if !account.CanTrade {
		return wrap.Errorf("account cannot trade")
	}
	usdt, err := helpers.FindUSDTBalance(account.Balances)
	if err != nil {
		return err
	}
	fmt.Printf("Свободный USDT: %.8f, заблокировано: %.8f\n", usdt.Free, usdt.Locked)

	unresolvedIntents, err := u.stateRepo.ListRecoverableOrderIntents(ctx)
	if err != nil {
		return err
	}
	if len(unresolvedIntents) > 0 {
		fmt.Printf("Торговля приостановлена: незавершённых order intents: %d. Запустите reconcile-orders\n", len(unresolvedIntents))
		return nil
	}

	openTrades, err := u.stateRepo.GetOpenOrders(ctx)
	if err != nil {
		return err
	}
	if len(openTrades) > 0 {
		for i := range openTrades {
			if err := u.reconcileTrade(ctx, openTrades[i], live); err != nil {
				return err
			}
		}
		return nil
	}

	if usdt.Free < signalOrderQuoteUSDT {
		fmt.Printf("Недостаточно свободного USDT для новой сделки: нужно %.2f\n", signalOrderQuoteUSDT)
		return nil
	}

	return u.openBestSignal(ctx, usdt.Free, live)
}

func (u *ExecutePalisadeSignals) openBestSignal(ctx context.Context, freeUSDT float64, live bool) error {
	signals, err := u.stateRepo.ListActivePalisadeSignals(ctx)
	if err != nil {
		return err
	}
	if len(signals) == 0 {
		fmt.Println("Активных сигналов нет")
		return nil
	}

	books, err := u.api.GetAllBookTickers(ctx)
	if err != nil {
		return wrap.Errorf("get book tickers: %w", err)
	}
	bookBySymbol := make(map[string]mexc.BookTicker, len(*books))
	for _, book := range *books {
		bookBySymbol[book.Symbol] = book
	}

	info, err := u.api.GetExchangeInfoAll(ctx)
	if err != nil {
		return wrap.Errorf("get exchange info: %w", err)
	}
	bySymbol := make(map[string]mexc.SymbolDetail, len(info.Symbols))
	for _, symbol := range info.Symbols {
		if symbol.IsSpotTradingAllowed && symbol.QuoteAsset == "USDT" {
			bySymbol[symbol.Symbol] = symbol
		}
	}

	now := time.Now().UTC()
	for _, signal := range signals {
		book, ok := bookBySymbol[signal.Symbol]
		if !ok {
			continue
		}
		bid, ask, err := parseBook(book)
		if err != nil || ask <= bid {
			continue
		}
		if ask < signal.EntryPrice || ask > signal.EntryPrice+(signal.TargetPrice-signal.EntryPrice)*0.30 {
			fmt.Printf("%s: цена вне зоны входа, bid=%.8f ask=%.8f entry=%.8f\n", signal.Symbol, bid, ask, signal.EntryPrice)
			continue
		}
		symbol, ok := bySymbol[signal.Symbol]
		if !ok {
			continue
		}
		step, err := swapLotStep(&symbol)
		if err != nil {
			fmt.Printf("%s: шаг количества: %v\n", signal.Symbol, err)
			continue
		}
		priceStep := signalPriceStep(&symbol)
		entryPrice := roundPriceDown(signal.EntryPrice, priceStep)
		quantity := swapRoundQtyDown(math.Min(signalOrderQuoteUSDT/entryPrice, freeUSDT/entryPrice), step)
		if quantity <= 0 || quantity*entryPrice < signalOrderQuoteUSDT*0.99 {
			fmt.Printf("%s: после округления объём слишком мал: %.8f\n", signal.Symbol, quantity)
			continue
		}

		targetPrice := roundPriceDown(signal.TargetPrice, priceStep)
		if targetPrice < signal.MinExitPrice {
			fmt.Printf("%s: цель после округления ниже минимального выхода\n", signal.Symbol)
			continue
		}
		signal.EntryPrice = entryPrice
		signal.TargetPrice = targetPrice
		if err := validateLimitOrder(symbol, order.BUY, entryPrice, quantity); err != nil {
			fmt.Printf("%s: BUY отклонён до отправки: %v\n", signal.Symbol, err)
			continue
		}
		fmt.Printf("Сигнал к исполнению: %s, BUY %.8f @ %.8f, цель %.8f\n", signal.Symbol, quantity, entryPrice, targetPrice)
		if !live {
			return nil
		}
		return u.placeBuy(ctx, signal, symbol, quantity, usdtBalanceSnapshot{free: freeUSDT}, now)
	}
	return nil
}

type usdtBalanceSnapshot struct{ free float64 }

func (u *ExecutePalisadeSignals) placeBuy(ctx context.Context, signal repo.PalisadeSignalState, symbol mexc.SymbolDetail, quantity float64, balance usdtBalanceSnapshot, now time.Time) error {
	if err := validateLimitOrder(symbol, order.BUY, signal.EntryPrice, quantity); err != nil {
		return wrap.Errorf("BUY constraints %s: %w", signal.Symbol, err)
	}
	clientID, err := newSignalClientOrderID(order.BUY)
	if err != nil {
		return err
	}
	intent, err := u.stateRepo.CreateOrderIntent(ctx, repo.OrderIntent{
		ClientOrderID: clientID,
		Symbol:        signal.Symbol,
		Side:          order.BUY.String(),
		Price:         signal.EntryPrice,
		Quantity:      quantity,
		OpenBalance:   balance.free,
		TargetPrice:   signal.TargetPrice,
		Status:        "PLACING",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		return err
	}
	result, err := u.api.NewOrder(model.OrderParams{
		Symbol:           signal.Symbol,
		Side:             order.BUY,
		OrderType:        order.LIMIT,
		Quantity:         quantity,
		QuoteOrderQty:    quantity,
		Price:            signal.EntryPrice,
		NewClientOrderId: clientID,
	})
	if err != nil {
		_ = u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "UNKNOWN", "", 0, 0, err.Error())
		return wrap.Errorf("place signal BUY %s: %w", signal.Symbol, err)
	}
	if result == nil || result.OrderID == "" {
		err = wrap.Errorf("empty order response")
		_ = u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "UNKNOWN", "", 0, 0, err.Error())
		return wrap.Errorf("place signal BUY %s: %w", signal.Symbol, err)
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "ACKNOWLEDGED", result.OrderID, 0, 0, ""); err != nil {
		return err
	}
	trade, err := u.stateRepo.SaveTradeLog(ctx, repo.SaveTradeLogParams{
		OpenDate:    now,
		OpenBalance: balance.free,
		Symbol:      signal.Symbol,
		BuyPrice:    signal.EntryPrice,
		Amount:      quantity,
		OrderId:     result.OrderID,
		UpLevel:     signal.TargetPrice,
		DownLevel:   signal.EntryPrice,
	})
	if err != nil {
		return wrap.Errorf("save signal BUY %s: %w", signal.Symbol, err)
	}
	if err := u.stateRepo.UpdateOrderIntentTradeID(ctx, intent.ID, trade.ID); err != nil {
		return err
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "LINKED", result.OrderID, 0, 0, ""); err != nil {
		return err
	}
	fmt.Printf("BUY размещён: %s, trade_log=%d, order=%s\n", signal.Symbol, trade.ID, result.OrderID)
	u.notify(fmt.Sprintf("<b>📥 Signal BUY</b> %s · <code>%s</code> · %.8f×%.8f · target %.8f", signal.Symbol, result.OrderID, signal.EntryPrice, quantity, signal.TargetPrice))
	return nil
}

func (u *ExecutePalisadeSignals) reconcileTrade(ctx context.Context, trade repo.TradeLog, live bool) error {
	orderID := trade.OrderId
	if trade.OrderId_sell != "" {
		orderID = trade.OrderId_sell
	}
	result, err := u.api.GetOrderQuery(trade.Symbol, orderID)
	if err != nil {
		return wrap.Errorf("query order %s/%s: %w", trade.Symbol, orderID, err)
	}
	if result == nil {
		return wrap.Errorf("order %s for %s is missing from exchange; manual recovery required", orderID, trade.Symbol)
	}
	if !strings.HasPrefix(result.ClientOrderID, "Signal_") {
		fmt.Printf("Пропуск старой сделки %s: clientOrderId=%s\n", trade.Symbol, result.ClientOrderID)
		return nil
	}

	if result.Side == order.BUY.String() {
		return u.reconcileBuy(ctx, trade, result, live)
	}
	if result.Side == order.SELL.String() {
		return u.reconcileSell(ctx, trade, result, live)
	}
	return wrap.Errorf("unknown order side %q for %s", result.Side, trade.Symbol)
}

func (u *ExecutePalisadeSignals) reconcileBuy(ctx context.Context, trade repo.TradeLog, result *mexc.QueryOrderResult, live bool) error {
	executed, quote, err := orderFill(result)
	if err != nil {
		return err
	}
	buyPrice := 0.0
	if executed > 0 {
		buyPrice = trade.BuyPrice
		if quote > 0 {
			buyPrice = quote / executed
		}
	}
	reason, _, err := u.emergencyExitReason(ctx, trade, buyPrice)
	if err != nil {
		return err
	}
	if reason != "" {
		fmt.Printf("Аварийный выход %s: %s\n", trade.Symbol, reason)
		if !live {
			return nil
		}
		if result.Status == "NEW" || result.Status == "PARTIALLY_FILLED" {
			if _, err := u.api.CancelOrder(trade.Symbol, trade.OrderId); err != nil {
				return wrap.Errorf("cancel BUY for emergency exit %s: %w", trade.Symbol, err)
			}
			result, err = u.api.GetOrderQuery(trade.Symbol, trade.OrderId)
			if err != nil || result == nil {
				return wrap.Errorf("query canceled BUY for emergency exit %s: %v", trade.Symbol, err)
			}
			executed, quote, err = orderFill(result)
			if err != nil {
				return err
			}
		}
		if executed <= 0 {
			if result.Status == "CANCELED" || result.Status == "PARTIALLY_CANCELED" {
				return u.stateRepo.UpdateCancelDateTradeLog(ctx, trade.ID, time.Now().UTC())
			}
			return nil
		}
		averageBuyPrice := trade.BuyPrice
		if quote > 0 {
			averageBuyPrice = quote / executed
		}
		if err := u.stateRepo.UpdateTradeFill(ctx, trade.ID, averageBuyPrice, executed); err != nil {
			return err
		}
		trade.BuyPrice = averageBuyPrice
		return u.placeEmergencySell(ctx, trade, executed, reason, live)
	}
	if result.Status == "NEW" && time.Since(trade.OpenDate) < signalBuyTimeout {
		fmt.Printf("BUY ожидает исполнения: %s %s\n", trade.Symbol, trade.OrderId)
		return nil
	}
	if result.Status == "NEW" && live {
		if _, err := u.api.CancelOrder(trade.Symbol, trade.OrderId); err != nil {
			return wrap.Errorf("cancel stale BUY %s: %w", trade.OrderId, err)
		}
		result, err = u.api.GetOrderQuery(trade.Symbol, trade.OrderId)
		if err != nil || result == nil {
			return wrap.Errorf("query canceled BUY %s: %v", trade.OrderId, err)
		}
		executed, quote, err = orderFill(result)
		if err != nil {
			return err
		}
	}
	if executed <= 0 {
		if result.Status == "CANCELED" || result.Status == "PARTIALLY_CANCELED" {
			return u.stateRepo.UpdateCancelDateTradeLog(ctx, trade.ID, time.Now().UTC())
		}
		return nil
	}
	averageBuyPrice := trade.BuyPrice
	if quote > 0 {
		averageBuyPrice = quote / executed
	}
	if err := u.stateRepo.UpdateTradeFill(ctx, trade.ID, averageBuyPrice, executed); err != nil {
		return err
	}
	trade.BuyPrice = averageBuyPrice
	if result.Status == "PARTIALLY_FILLED" && live {
		if _, err := u.api.CancelOrder(trade.Symbol, trade.OrderId); err != nil {
			return wrap.Errorf("cancel partially filled BUY %s: %w", trade.OrderId, err)
		}
		result, err = u.api.GetOrderQuery(trade.Symbol, trade.OrderId)
		if err != nil || result == nil {
			return wrap.Errorf("query canceled partial BUY %s: %v", trade.OrderId, err)
		}
		executed, quote, err = orderFill(result)
		if err != nil {
			return err
		}
		if executed > 0 {
			averageBuyPrice := trade.BuyPrice
			if quote > 0 {
				averageBuyPrice = quote / executed
			}
			if err := u.stateRepo.UpdateTradeFill(ctx, trade.ID, averageBuyPrice, executed); err != nil {
				return err
			}
			trade.BuyPrice = averageBuyPrice
		}
	}
	return u.placeSellForTrade(ctx, trade, executed, live)
}

func (u *ExecutePalisadeSignals) reconcileSell(ctx context.Context, trade repo.TradeLog, result *mexc.QueryOrderResult, live bool) error {
	executed, quote, err := orderFill(result)
	if err != nil {
		return err
	}
	if err := u.recordSellOrderState(ctx, trade.ID, result, executed, quote); err != nil {
		return err
	}
	if result.Status == "FILLED" {
		return u.replaceSellRemainder(ctx, trade, live, 0, "")
	}
	reason, _, err := u.emergencyExitReason(ctx, trade, trade.BuyPrice)
	if err != nil {
		return err
	}
	if reason != "" {
		fmt.Printf("Аварийный выход %s: %s\n", trade.Symbol, reason)
		if !live {
			return nil
		}
		if result.Status == "NEW" || result.Status == "PARTIALLY_FILLED" {
			_, err = u.cancelAndRefreshSell(ctx, trade, result)
			if err != nil {
				return wrap.Errorf("cancel SELL for emergency exit %s: %w", trade.Symbol, err)
			}
		}
		progress, err := u.getSellProgress(ctx, trade.ID)
		if err != nil {
			return err
		}
		if progress.remaining() <= sellQuantityTolerance(progress.planned) {
			return u.finishSell(ctx, trade)
		}
		return u.placeEmergencySell(ctx, trade, progress.remaining(), reason, live)
	}
	if result.Status == "NEW" {
		return u.maybeRepriceSell(ctx, trade, result, live)
	}
	if result.Status == "PARTIALLY_FILLED" {
		fmt.Printf("SELL частично исполнен: %s, qty %.8f\n", trade.Symbol, executed)
		return u.maybeRepriceSell(ctx, trade, result, live)
	}
	if result.Status == "CANCELED" || result.Status == "PARTIALLY_CANCELED" ||
		result.Status == "REJECTED" || result.Status == "EXPIRED" {
		if !live {
			fmt.Printf("SELL отменён/частично отменён: %s, исполнено %.8f\n", trade.Symbol, executed)
			return nil
		}
		return u.replaceSellRemainder(ctx, trade, true, 0, "")
	}
	return nil
}

func (u *ExecutePalisadeSignals) finishSell(ctx context.Context, trade repo.TradeLog) error {
	progress, err := u.getSellProgress(ctx, trade.ID)
	if err != nil {
		return err
	}
	if progress.executed <= 0 || progress.quote <= 0 {
		return wrap.Errorf("SELL for trade %d has no aggregate execution data", trade.ID)
	}
	closeAt := time.Now().UTC()
	if err := u.stateRepo.UpdateSuccesTradeLog(ctx, trade.ID, closeAt, progress.quote, progress.quote/progress.executed); err != nil {
		return err
	}
	fmt.Printf("SELL завершён: %s, qty %.8f, %.8f USDT\n", trade.Symbol, progress.executed, progress.quote)
	u.notify(fmt.Sprintf("<b>💰 Signal SELL</b> %s · qty %.8f · %.8f USDT", trade.Symbol, progress.executed, progress.quote))
	return nil
}

func (u *ExecutePalisadeSignals) maybeRepriceSell(ctx context.Context, trade repo.TradeLog, current *mexc.QueryOrderResult, live bool) error {
	if !live {
		return nil
	}
	signals, err := u.stateRepo.ListActivePalisadeSignals(ctx)
	if err != nil {
		return err
	}
	for _, signal := range signals {
		if signal.Symbol != trade.Symbol || signal.TargetPrice <= 0 {
			continue
		}
		if math.Abs(signal.TargetPrice-trade.UpLevel)/trade.UpLevel < signalRepriceDelta {
			return nil
		}
		if signal.TargetPrice < signal.MinExitPrice {
			return nil
		}
		_, err := u.cancelAndRefreshSell(ctx, trade, current)
		if err != nil {
			return wrap.Errorf("cancel outdated SELL %s: %w", trade.OrderId_sell, err)
		}
		if err := u.stateRepo.UpdateTradeLevels(ctx, trade.ID, signal.TargetPrice, trade.DownLevel); err != nil {
			return err
		}
		trade.UpLevel = signal.TargetPrice
		return u.replaceSellRemainder(ctx, trade, true, 0, "")
	}
	return nil
}

func (u *ExecutePalisadeSignals) cancelAndRefreshSell(
	ctx context.Context,
	trade repo.TradeLog,
	current *mexc.QueryOrderResult,
) (*mexc.QueryOrderResult, error) {
	orderID := trade.OrderId_sell
	if current != nil && current.OrderID != "" {
		orderID = current.OrderID
	}
	_, cancelErr := u.api.CancelOrder(trade.Symbol, orderID)
	finalOrder, queryErr := u.api.GetOrderQuery(trade.Symbol, orderID)
	if queryErr != nil {
		return nil, wrap.Errorf("query SELL %s after cancel: %w", orderID, queryErr)
	}
	if finalOrder == nil {
		if cancelErr != nil {
			return nil, wrap.Errorf("cancel SELL %s: %w", orderID, cancelErr)
		}
		return nil, wrap.Errorf("SELL %s is missing after cancel", orderID)
	}
	executed, quote, err := orderFill(finalOrder)
	if err != nil {
		return nil, err
	}
	if err := u.recordSellOrderState(ctx, trade.ID, finalOrder, executed, quote); err != nil {
		return nil, err
	}
	if finalOrder.Status == "NEW" || finalOrder.Status == "PARTIALLY_FILLED" {
		if cancelErr != nil {
			return nil, wrap.Errorf("cancel SELL %s: %w", orderID, cancelErr)
		}
		return nil, wrap.Errorf("SELL %s is still active after cancel, status=%s", orderID, finalOrder.Status)
	}
	return finalOrder, nil
}

func (u *ExecutePalisadeSignals) replaceSellRemainder(
	ctx context.Context,
	trade repo.TradeLog,
	live bool,
	forcedPrice float64,
	reason string,
) error {
	progress, err := u.getSellProgress(ctx, trade.ID)
	if err != nil {
		return err
	}
	remaining := progress.remaining()
	if remaining <= sellQuantityTolerance(progress.planned) {
		return u.finishSell(ctx, trade)
	}
	return u.placeSellForTradeAtPrice(ctx, trade, remaining, live, forcedPrice, reason)
}

func (u *ExecutePalisadeSignals) placeEmergencySell(ctx context.Context, trade repo.TradeLog, requested float64, reason string, live bool) error {
	quote, err := u.getMarketQuote(ctx, trade.Symbol)
	if err != nil {
		return err
	}
	if quote.bid <= 0 || quote.ask <= quote.bid || (quote.ask-quote.bid)/quote.bid > maxEmergencySpread {
		u.notify(fmt.Sprintf("<b>⚠️ Emergency exit delayed</b> %s · %s · spread %.3f%%", trade.Symbol, reason, (quote.ask/quote.bid-1)*100))
		fmt.Printf("Аварийная продажа отложена: %s, спред слишком широк или стакан некорректен\n", trade.Symbol)
		return nil
	}
	price := quote.bid * (1 - emergencyPriceDiscount)
	return u.placeSellForTradeAtPrice(ctx, trade, requested, live, price, "EMERGENCY: "+reason)
}

func (u *ExecutePalisadeSignals) placeSellForTrade(ctx context.Context, trade repo.TradeLog, requested float64, live bool) error {
	return u.placeSellForTradeAtPrice(ctx, trade, requested, live, 0, "")
}

func (u *ExecutePalisadeSignals) placeSellForTradeAtPrice(ctx context.Context, trade repo.TradeLog, requested float64, live bool, forcedPrice float64, reason string) error {
	account, err := u.api.GetBalance(ctx)
	if err != nil {
		return err
	}
	info, err := u.api.GetExchangeInfoAll(ctx)
	if err != nil {
		return err
	}
	var symbol *mexc.SymbolDetail
	for i := range info.Symbols {
		if info.Symbols[i].Symbol == trade.Symbol {
			symbol = &info.Symbols[i]
			break
		}
	}
	if symbol == nil {
		return wrap.Errorf("symbol %s missing from exchange info", trade.Symbol)
	}
	balance, err := helpers.FindAssetBalance(account.Balances, symbol.BaseAsset)
	if err != nil {
		return err
	}
	step, err := swapLotStep(symbol)
	if err != nil {
		return err
	}
	quantity := swapRoundQtyDown(math.Min(requested, balance.Free), step)
	if quantity <= 0 {
		return wrap.Errorf("no sellable %s balance for %s", symbol.BaseAsset, trade.Symbol)
	}
	price := roundPriceDown(trade.UpLevel, signalPriceStep(symbol))
	if forcedPrice > 0 {
		price = roundPriceDown(forcedPrice, signalPriceStep(symbol))
	}
	if price <= 0 {
		return wrap.Errorf("rounded sell price for %s is zero", trade.Symbol)
	}
	if err := validateLimitOrder(*symbol, order.SELL, price, quantity); err != nil {
		return wrap.Errorf("SELL constraints %s: %w", trade.Symbol, err)
	}
	fmt.Printf("SELL к размещению: %s %.8f @ %.8f\n", trade.Symbol, quantity, price)
	if !live {
		return nil
	}
	clientID, err := newSignalClientOrderID(order.SELL)
	if err != nil {
		return err
	}
	intent, err := u.stateRepo.CreateOrderIntent(ctx, repo.OrderIntent{
		ClientOrderID: clientID,
		Symbol:        trade.Symbol,
		Side:          order.SELL.String(),
		Price:         price,
		Quantity:      quantity,
		TargetPrice:   price,
		LastError:     reason,
		Status:        "PLACING",
		TradeID:       trade.ID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	result, err := u.api.NewOrder(model.OrderParams{
		Symbol:           trade.Symbol,
		Side:             order.SELL,
		OrderType:        order.LIMIT,
		Quantity:         quantity,
		QuoteOrderQty:    quantity,
		Price:            price,
		NewClientOrderId: clientID,
	})
	if err != nil {
		_ = u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "UNKNOWN", "", 0, 0, err.Error())
		return wrap.Errorf("place signal SELL %s: %w", trade.Symbol, err)
	}
	if result == nil || result.OrderID == "" {
		err = wrap.Errorf("empty order response")
		_ = u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "UNKNOWN", "", 0, 0, err.Error())
		return wrap.Errorf("place signal SELL %s: %w", trade.Symbol, err)
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "ACKNOWLEDGED", result.OrderID, 0, 0, ""); err != nil {
		return err
	}
	if err := u.stateRepo.UpdateSellOrderIdTradeLog(ctx, trade.ID, result.OrderID); err != nil {
		return err
	}
	if err := u.stateRepo.UpdateDealDateTradeLog(ctx, trade.ID, time.Now().UTC()); err != nil {
		return err
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "LINKED", result.OrderID, 0, 0, ""); err != nil {
		return err
	}
	fmt.Printf("SELL размещён: %s order=%s qty=%.8f\n", trade.Symbol, result.OrderID, quantity)
	if reason != "" {
		u.notify(fmt.Sprintf("<b>🚨 Emergency SELL</b> %s · <code>%s</code> · %.8f×%.8f · %s", trade.Symbol, result.OrderID, price, quantity, reason))
	} else {
		u.notify(fmt.Sprintf("<b>📤 Signal SELL</b> %s · <code>%s</code> · %.8f×%.8f", trade.Symbol, result.OrderID, price, quantity))
	}
	return nil
}

type sellProgress struct {
	planned  float64
	executed float64
	quote    float64
}

func (p sellProgress) remaining() float64 {
	return math.Max(0, p.planned-p.executed)
}

func sellQuantityTolerance(planned float64) float64 {
	return 1e-10 * math.Max(1, planned)
}

func summarizeSellIntents(intents []repo.OrderIntent) (sellProgress, error) {
	progress := sellProgress{}
	for _, intent := range intents {
		if intent.Side != order.SELL.String() {
			continue
		}
		if progress.planned == 0 {
			progress.planned = intent.Quantity
		}
		if intent.ExecutedQuantity < 0 || intent.CumulativeQuoteQty < 0 {
			return sellProgress{}, wrap.Errorf("negative SELL execution in intent %d", intent.ID)
		}
		progress.executed += intent.ExecutedQuantity
		progress.quote += intent.CumulativeQuoteQty
	}
	if progress.planned <= 0 {
		return sellProgress{}, wrap.Errorf("SELL intents have no planned quantity")
	}
	if progress.executed > progress.planned+sellQuantityTolerance(progress.planned) {
		return sellProgress{}, wrap.Errorf(
			"aggregate SELL quantity %.12f exceeds planned position %.12f",
			progress.executed,
			progress.planned,
		)
	}
	return progress, nil
}

func (u *ExecutePalisadeSignals) getSellProgress(ctx context.Context, tradeID int) (sellProgress, error) {
	intents, err := u.stateRepo.ListOrderIntentsByTradeID(ctx, tradeID)
	if err != nil {
		return sellProgress{}, err
	}
	progress, err := summarizeSellIntents(intents)
	if err != nil {
		return sellProgress{}, wrap.Errorf("summarize SELL intents for trade %d: %w", tradeID, err)
	}
	return progress, nil
}

func (u *ExecutePalisadeSignals) recordSellOrderState(
	ctx context.Context,
	tradeID int,
	result *mexc.QueryOrderResult,
	executed float64,
	quote float64,
) error {
	intents, err := u.stateRepo.ListOrderIntentsByTradeID(ctx, tradeID)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		if intent.Side != order.SELL.String() {
			continue
		}
		if intent.ExchangeOrderID != result.OrderID && intent.ClientOrderID != result.ClientOrderID {
			continue
		}
		return u.stateRepo.UpdateOrderIntent(ctx, intent.ID, result.Status, result.OrderID, executed, quote, "")
	}
	return wrap.Errorf("SELL intent for trade %d and order %s not found", tradeID, result.OrderID)
}

type marketQuote struct {
	bid, bidQty, ask, askQty float64
}

func (u *ExecutePalisadeSignals) getMarketQuote(ctx context.Context, symbol string) (marketQuote, error) {
	books, err := u.api.GetAllBookTickers(ctx)
	if err != nil {
		return marketQuote{}, wrap.Errorf("get book ticker for %s: %w", symbol, err)
	}
	for _, book := range *books {
		if book.Symbol != symbol {
			continue
		}
		bid, ask, err := parseBook(book)
		if err != nil {
			return marketQuote{}, err
		}
		bidQty, _ := strconv.ParseFloat(book.BidQty, 64)
		askQty, _ := strconv.ParseFloat(book.AskQty, 64)
		return marketQuote{bid: bid, bidQty: bidQty, ask: ask, askQty: askQty}, nil
	}
	return marketQuote{}, wrap.Errorf("book ticker for %s not found", symbol)
}

func (u *ExecutePalisadeSignals) emergencyExitReason(ctx context.Context, trade repo.TradeLog, buyPrice float64) (string, marketQuote, error) {
	quote, err := u.getMarketQuote(ctx, trade.Symbol)
	if err != nil {
		return "", marketQuote{}, err
	}
	if quote.bid <= 0 {
		return "", quote, nil
	}
	openedAt := trade.OpenDate
	if trade.DealDate != nil {
		openedAt = *trade.DealDate
	}
	return emergencyReason(time.Now().UTC(), openedAt, quote.bid, trade.DownLevel, buyPrice), quote, nil
}

func emergencyReason(now, openedAt time.Time, bid, support, buyPrice float64) string {
	if support > 0 && bid < support*(1-supportBreakPercent) {
		return "SUPPORT_BROKEN"
	}
	if buyPrice > 0 && bid < buyPrice*(1-hardLossPercent) {
		return "MAX_LOSS"
	}
	if buyPrice > 0 && now.Sub(openedAt) > maxPositionHold {
		return "MAX_HOLD_TIME"
	}
	return ""
}

func orderFill(orderInfo *mexc.QueryOrderResult) (executed, quote float64, err error) {
	if orderInfo == nil {
		return 0, 0, wrap.Errorf("order response is nil")
	}
	executed, err = strconv.ParseFloat(orderInfo.ExecutedQty, 64)
	if err != nil {
		return 0, 0, wrap.Errorf("parse executed quantity %q: %w", orderInfo.ExecutedQty, err)
	}
	quote, err = strconv.ParseFloat(orderInfo.CummulativeQuoteQty, 64)
	if err != nil {
		return 0, 0, wrap.Errorf("parse cumulative quote quantity %q: %w", orderInfo.CummulativeQuoteQty, err)
	}
	return executed, quote, nil
}

func newSignalClientOrderID(side order.Side) (string, error) {
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", wrap.Errorf("generate client order id: %w", err)
	}
	sideCode := "B"
	if side == order.SELL {
		sideCode = "S"
	}
	return "Signal_" + sideCode + "_" + hex.EncodeToString(entropy[:]), nil
}

func signalPriceStep(symbol *mexc.SymbolDetail) float64 {
	for _, filter := range symbol.Filters {
		if filter.FilterType != "PRICE_FILTER" {
			continue
		}
		if step, err := strconv.ParseFloat(filter.StepSize, 64); err == nil && step > 0 {
			return step
		}
	}
	if symbol.QuotePrecision > 0 {
		return math.Pow(10, -float64(symbol.QuotePrecision))
	}
	return 0.00000001
}

func roundPriceDown(price, step float64) float64 {
	if price <= 0 || step <= 0 {
		return 0
	}
	return math.Floor(price/step) * step
}

func roundPriceUp(price, step float64) float64 {
	if step <= 0 {
		return price
	}
	return math.Ceil(price/step-1e-12) * step
}

func parseBook(book mexc.BookTicker) (bid, ask float64, err error) {
	bid, err = strconv.ParseFloat(book.BidPrice, 64)
	if err != nil {
		return 0, 0, err
	}
	ask, err = strconv.ParseFloat(book.AskPrice, 64)
	if err != nil {
		return 0, 0, err
	}
	return bid, ask, nil
}

func (u *ExecutePalisadeSignals) notify(message string) {
	if u.telegram == nil || !u.telegram.Configured() {
		return
	}
	if _, err := u.telegram.Send(message); err != nil {
		fmt.Printf("Telegram: %v\n", err)
	}
}

func validateLimitOrder(symbol mexc.SymbolDetail, side order.Side, price, quantity float64) error {
	if !symbol.IsSpotTradingAllowed {
		return fmt.Errorf("spot trading is not allowed")
	}
	if symbol.Status != "" && symbol.Status != "1" && !strings.EqualFold(symbol.Status, "online") {
		return fmt.Errorf("symbol status is %s", symbol.Status)
	}
	if symbol.TradeSideType == 2 && side != order.BUY {
		return fmt.Errorf("only BUY orders are allowed")
	}
	if symbol.TradeSideType == 3 && side != order.SELL {
		return fmt.Errorf("only SELL orders are allowed")
	}
	if len(symbol.OrderTypes) > 0 && !supportsLimitOrder(symbol.OrderTypes) {
		return fmt.Errorf("LIMIT order type is not supported")
	}
	if price <= 0 || quantity <= 0 {
		return fmt.Errorf("price and quantity must be positive")
	}
	priceStep := signalPriceStep(&symbol)
	if !stepAligned(price, priceStep) {
		return fmt.Errorf("price %.12f is not aligned to step %.12f", price, priceStep)
	}
	quantityStep, err := swapLotStep(&symbol)
	if err != nil {
		return err
	}
	if !stepAligned(quantity, quantityStep) {
		return fmt.Errorf("quantity %.12f is not aligned to step %.12f", quantity, quantityStep)
	}
	quoteAmount := price * quantity
	for _, filter := range symbol.Filters {
		if filter.FilterType != "LOT_SIZE" {
			continue
		}
		minQty := parsePositiveFloat(filter.MinQty)
		maxQty := parsePositiveFloat(filter.MaxQty)
		if minQty > 0 && quantity < minQty {
			return fmt.Errorf("quantity %.12f is below minQty %.12f", quantity, minQty)
		}
		if maxQty > 0 && quantity > maxQty {
			return fmt.Errorf("quantity %.12f is above maxQty %.12f", quantity, maxQty)
		}
	}
	minQuote := parsePositiveFloat(symbol.QuoteAmountPrecision)
	maxQuote := parsePositiveFloat(symbol.MaxQuoteAmount)
	if minQuote > 0 && quoteAmount < minQuote {
		return fmt.Errorf("quote amount %.12f is below minimum %.12f", quoteAmount, minQuote)
	}
	if maxQuote > 0 && quoteAmount > maxQuote {
		return fmt.Errorf("quote amount %.12f is above maximum %.12f", quoteAmount, maxQuote)
	}
	return nil
}

func supportsLimitOrder(orderTypes []string) bool {
	for _, orderType := range orderTypes {
		if strings.EqualFold(orderType, "LIMIT") || strings.EqualFold(orderType, "LIMIT_ORDER") {
			return true
		}
	}
	return false
}

func parsePositiveFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func stepAligned(value, step float64) bool {
	if value <= 0 || step <= 0 {
		return false
	}
	ratio := value / step
	return math.Abs(ratio-math.Round(ratio)) <= 1e-8*math.Max(1, math.Abs(ratio))
}
