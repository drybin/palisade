package usecase

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const (
	paperLockKey             = "palisade:paper-trading"
	paperStrategyVersion     = 7
	maxPaperOpenTrades       = 1
	paperTrailingTrigger     = 0.004
	paperTrailingDistance    = 0.0025
	paperMinimumLockedProfit = 0.00025
	paperEntryRunawayPercent = 0.004
	paperPullbackTimeout     = 30 * time.Minute
)

type IPaperTrade interface {
	Process(context.Context, bool) error
}

type PaperTradeRunner struct {
	api       *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewPaperTradeUsecase(api *webapi.MexcWebapi, stateRepo repo.IStateRepository) *PaperTradeRunner {
	return &PaperTradeRunner{api: api, stateRepo: stateRepo}
}

func (u *PaperTradeRunner) Process(ctx context.Context, debug bool) error {
	releaseLock, acquired, err := acquireNamedLock(ctx, u.stateRepo, paperLockKey)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer releaseLock()

	signals, err := u.stateRepo.ListActivePalisadeSignals(ctx)
	if err != nil {
		return err
	}
	books, err := u.api.GetAllBookTickers(ctx)
	if err != nil {
		return wrap.Errorf("get book tickers for paper trading: %w", err)
	}
	bookBySymbol := make(map[string]mexc.BookTicker, len(*books))
	for _, book := range *books {
		bookBySymbol[book.Symbol] = book
	}
	info, err := u.api.GetExchangeInfoAll(ctx)
	if err != nil {
		return wrap.Errorf("get exchange info for paper trading: %w", err)
	}
	bySymbol := make(map[string]mexc.SymbolDetail, len(info.Symbols))
	for _, symbol := range info.Symbols {
		if symbol.Symbol != "" && symbol.QuoteAsset == "USDT" {
			bySymbol[symbol.Symbol] = symbol
		}
	}

	openTrades, err := u.stateRepo.ListOpenPaperTrades(ctx, paperStrategyVersion)
	if err != nil {
		return err
	}
	legacyOpenTrades, err := u.stateRepo.ListOpenPaperTrades(ctx, paperStrategyVersion-1)
	if err != nil {
		return err
	}
	openTrades = append(legacyOpenTrades, openTrades...)
	openBySymbol := make(map[string]repo.PaperTrade, len(openTrades))
	for _, trade := range openTrades {
		if trade.StrategyVersion == paperStrategyVersion {
			openBySymbol[trade.Symbol] = trade
		}
	}

	openSlotsUsed := 0
	for i := range openTrades {
		trade := openTrades[i]
		if err := u.processPaperTrade(ctx, &trade, signalFor(signals, trade.Symbol), bookBySymbol[trade.Symbol], bySymbol[trade.Symbol], time.Now().UTC()); err != nil {
			return err
		}
		if trade.StrategyVersion == paperStrategyVersion && isOpenPaperTrade(trade) {
			openSlotsUsed++
		}
		if debug {
			fmt.Printf("paper %s: status=%s mode=%s support=%.8f break_even=%t max_bid=%.8f min_bid=%.8f filled=%.8f sold=%.8f mark_pnl=%.8f\n", trade.Symbol, trade.Status, trade.EntryMode, trade.SupportPrice, trade.BreakEvenArmed, trade.MaxBidPrice, trade.MinBidPrice, trade.FilledQuantity, trade.SoldQuantity, trade.PnL)
		}
	}

	created := 0
	for _, signal := range signals {
		if openSlotsUsed >= maxPaperOpenTrades {
			break
		}
		if signal.StrategyVersion != paperStrategyVersion {
			continue
		}
		if _, exists := openBySymbol[signal.Symbol]; exists {
			continue
		}
		signalAt := paperSignalAt(signal)
		if signalAt.IsZero() {
			if debug {
				fmt.Printf("paper %s: у сигнала нет sent_at\n", signal.Symbol)
			}
			continue
		}
		existing, err := u.stateRepo.GetPaperTradeBySignal(ctx, signal.Symbol, signalAt, paperStrategyVersion)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		book, ok := bookBySymbol[signal.Symbol]
		symbol, symbolOK := bySymbol[signal.Symbol]
		if !ok || !symbolOK {
			continue
		}
		trade, ok, err := buildPaperTrade(signal, book, symbol, time.Now().UTC())
		if err != nil {
			if debug {
				fmt.Printf("paper %s: %v\n", signal.Symbol, err)
			}
			continue
		}
		if !ok {
			continue
		}
		createdTrade, err := u.stateRepo.CreatePaperTrade(ctx, trade)
		if err != nil {
			return err
		}
		created++
		openSlotsUsed++
		if err := u.processPaperTrade(ctx, createdTrade, signal, book, symbol, time.Now().UTC()); err != nil {
			return err
		}
	}

	stats, err := u.stateRepo.GetPaperTradeStats(ctx, paperStrategyVersion)
	if err != nil {
		return err
	}
	fmt.Printf("Paper trading v%d: открытых=%d, новых=%d, закрыто=%d, отменено=%d, всего=%d, P/L закрытых=%.8f USDT, P/L открытых=%.8f USDT, win=%d, loss=%d\n",
		paperStrategyVersion, stats.Open, created, stats.Closed, stats.Canceled, stats.Total,
		stats.TotalPnL, stats.OpenPnL, stats.Wins, stats.Losses)
	return nil
}

func isOpenPaperTrade(trade repo.PaperTrade) bool {
	return trade.Status == "BUY_PENDING" || trade.Status == "POSITION_OPEN" || trade.Status == "SELL_PENDING"
}

func buildPaperTrade(signal repo.PalisadeSignalState, book mexc.BookTicker, symbol mexc.SymbolDetail, now time.Time) (repo.PaperTrade, bool, error) {
	bid, ask, err := parseBook(book)
	if err != nil || ask <= bid || ask <= 0 {
		return repo.PaperTrade{}, false, nil
	}
	priceStep := signalPriceStep(&symbol)
	entry := roundPriceDown(signal.EntryPrice, priceStep)
	support := roundPriceDown(signal.SupportPrice, priceStep)
	if support <= 0 {
		support = entry
	}
	step, err := swapLotStep(&symbol)
	if err != nil {
		return repo.PaperTrade{}, false, err
	}
	quantity := swapRoundQtyDown(signalOrderQuoteUSDT/entry, step)
	if quantity <= 0 || !isValidPaperOrder(symbol, order.BUY, entry, quantity) {
		return repo.PaperTrade{}, false, nil
	}
	return repo.PaperTrade{
		StrategyVersion:   paperStrategyVersion,
		Symbol:            signal.Symbol,
		SignalAt:          paperSignalAt(signal),
		Status:            "BUY_PENDING",
		EntryMode:         "REBOUND_PULLBACK_TRAILING_V7",
		SupportPrice:      support,
		EntryPrice:        entry,
		TargetPrice:       roundPriceDown(signal.TargetPrice, signalPriceStep(&symbol)),
		MinExitPrice:      roundPriceDown(signal.MinExitPrice, signalPriceStep(&symbol)),
		ExpectedNetProfit: signal.NetProfit,
		Quantity:          quantity,
		LastPrice:         (bid + ask) / 2,
		UpdatedAt:         now,
	}, true, nil
}

func (u *PaperTradeRunner) processPaperTrade(ctx context.Context, trade *repo.PaperTrade, signal repo.PalisadeSignalState, book mexc.BookTicker, symbol mexc.SymbolDetail, now time.Time) error {
	if book.Symbol == "" || symbol.Symbol == "" {
		return nil
	}
	bid, ask, err := parseBook(book)
	if err != nil {
		return err
	}
	bidQty, _ := parseDecimalValue(book.BidQty)
	askQty, _ := parseDecimalValue(book.AskQty)
	trade.LastPrice = (bid + ask) / 2
	fee := math.Max(parseDecimal(symbol.MakerCommission), parseDecimal(symbol.TakerCommission))

	if signal.Symbol == trade.Symbol && signal.StrategyVersion == trade.StrategyVersion && signal.TargetPrice >= signal.MinExitPrice {
		updatePaperTarget(trade, signal.TargetPrice, signalPriceStep(&symbol))
	}

	if trade.Status == "BUY_PENDING" {
		entryCancelReason := paperEntryCancelReason(*trade, now, bid)
		if entryCancelReason != "" {
			if trade.FilledQuantity == 0 {
				trade.Status = "CANCELED"
				trade.ExitReason = entryCancelReason
				return u.persistPaperTrade(ctx, trade, now, bid, fee)
			}
			trade.Status = "POSITION_OPEN"
		}
		if trade.Status == "BUY_PENDING" && ask <= trade.EntryPrice {
			remaining := trade.Quantity - trade.FilledQuantity
			step, stepErr := swapLotStep(&symbol)
			if stepErr != nil {
				return stepErr
			}
			fillQty := swapRoundQtyDown(paperFillQuantity(remaining, askQty), step)
			if fillQty > 0 {
				fillPrice := math.Min(ask, trade.EntryPrice)
				trade.FilledQuantity += fillQty
				trade.BuyQuote += fillPrice * fillQty
				trade.Fees += fillPrice * fillQty * fee
				opened := now
				trade.OpenedAt = &opened
				trade.Status = "POSITION_OPEN"
			}
		}
	}

	if trade.Status == "POSITION_OPEN" || trade.Status == "SELL_PENDING" {
		if trade.FilledQuantity <= trade.SoldQuantity {
			trade.Status = "CLOSED"
			trade.ExitReason = "NO_REMAINING_ASSET"
			closed := now
			trade.ClosedAt = &closed
			return u.persistPaperTrade(ctx, trade, now, bid, fee)
		}
		buyPrice := trade.BuyQuote / trade.FilledQuantity
		trackPaperExcursion(trade, bid)
		support := trade.SupportPrice
		if support <= 0 {
			support = trade.EntryPrice
		}
		if trade.StrategyVersion >= 7 && !trade.BreakEvenArmed && bid >= paperTrailingActivationPrice(buyPrice, fee) {
			trade.BreakEvenArmed = true
		} else if trade.StrategyVersion >= 4 && trade.StrategyVersion < 7 && !trade.BreakEvenArmed && bid >= paperBreakEvenTrigger(buyPrice, trade.TargetPrice, fee) {
			trade.BreakEvenArmed = true
		}
		reason := paperExitReason(*trade, now, bid, support, buyPrice, fee)
		shouldSell := reason != "" || bid >= trade.TargetPrice
		if shouldSell {
			if reason == "" {
				reason = "TARGET_REACHED"
			}
			remaining := trade.FilledQuantity - trade.SoldQuantity
			fillQty := paperFillQuantity(remaining, bidQty)
			if fillQty > 0 {
				fillPrice := bid
				if reason != "TARGET_REACHED" {
					fillPrice = bid * (1 - emergencyPriceDiscount)
				}
				trade.SoldQuantity += fillQty
				trade.SellQuote += fillPrice * fillQty
				trade.Fees += fillPrice * fillQty * fee
				trade.ExitReason = reason
				trade.Status = "SELL_PENDING"
			}
		}
		if trade.SoldQuantity >= trade.FilledQuantity-1e-12 {
			trade.SoldQuantity = trade.FilledQuantity
			trade.Status = "CLOSED"
			closed := now
			trade.ClosedAt = &closed
		}
	}
	return u.persistPaperTrade(ctx, trade, now, bid, fee)
}

func updatePaperTarget(trade *repo.PaperTrade, target, priceStep float64) {
	updatedTarget := roundPriceDown(target, priceStep)
	if trade.StrategyVersion < 7 || updatedTarget > trade.TargetPrice {
		trade.TargetPrice = updatedTarget
	}
}

func paperEntryCancelReason(trade repo.PaperTrade, now time.Time, bid float64) string {
	if trade.StrategyVersion < 7 {
		if bid < trade.EntryPrice*(1-supportBreakPercent) || now.Sub(trade.SignalAt) > signalBuyTimeout {
			return "BUY_NOT_FILLED"
		}
		return ""
	}
	if bid < trade.SupportPrice*(1-supportBreakPercent) {
		return "SUPPORT_BROKEN_BEFORE_ENTRY"
	}
	if now.Sub(trade.SignalAt) > paperPullbackTimeout {
		return "PULLBACK_TIMEOUT"
	}
	if bid > trade.EntryPrice*(1+paperEntryRunawayPercent) {
		return "ENTRY_RAN_AWAY"
	}
	return ""
}

func paperExitReason(trade repo.PaperTrade, now time.Time, bid, support, buyPrice, fee float64) string {
	if trade.StrategyVersion >= 7 && trade.BreakEvenArmed && bid <= paperTrailingStopPrice(trade, buyPrice, fee) {
		return "TRAILING_STOP"
	}
	if trade.StrategyVersion >= 4 && trade.BreakEvenArmed && bid <= paperBreakEvenBidPrice(buyPrice, fee) {
		return "BREAKEVEN_STOP"
	}
	return emergencyReason(now, paperOpenedAt(trade), bid, support, buyPrice)
}

func trackPaperExcursion(trade *repo.PaperTrade, bid float64) {
	if bid <= 0 {
		return
	}
	if trade.MaxBidPrice <= 0 || bid > trade.MaxBidPrice {
		trade.MaxBidPrice = bid
	}
	if trade.MinBidPrice <= 0 || bid < trade.MinBidPrice {
		trade.MinBidPrice = bid
	}
}

func paperBreakEvenTrigger(buyPrice, targetPrice, fee float64) float64 {
	trigger := buyPrice + (targetPrice-buyPrice)*0.35
	return math.Max(trigger, paperBreakEvenBidPrice(buyPrice, fee)*1.0005)
}

func paperTrailingActivationPrice(buyPrice, fee float64) float64 {
	return math.Max(buyPrice*(1+paperTrailingTrigger), paperPositiveStopBidPrice(buyPrice, fee))
}

func paperTrailingStopPrice(trade repo.PaperTrade, buyPrice, fee float64) float64 {
	trailing := trade.MaxBidPrice * (1 - paperTrailingDistance)
	return math.Max(trailing, paperPositiveStopBidPrice(buyPrice, fee))
}

func paperPositiveStopBidPrice(buyPrice, fee float64) float64 {
	if buyPrice <= 0 || fee < 0 || fee >= 1 {
		return math.Inf(1)
	}
	return buyPrice * (1 + fee) * (1 + paperMinimumLockedProfit) / ((1 - emergencyPriceDiscount) * (1 - fee))
}

func paperBreakEvenBidPrice(buyPrice, fee float64) float64 {
	if buyPrice <= 0 || fee < 0 || fee >= 1 {
		return math.Inf(1)
	}
	return buyPrice * (1 + fee) / ((1 - emergencyPriceDiscount) * (1 - fee))
}

func (u *PaperTradeRunner) persistPaperTrade(
	ctx context.Context,
	trade *repo.PaperTrade,
	now time.Time,
	bid float64,
	fee float64,
) error {
	markPaperPnL(trade, bid, fee)
	trade.UpdatedAt = now
	return u.stateRepo.UpdatePaperTrade(ctx, *trade)
}

func markPaperPnL(trade *repo.PaperTrade, bid, fee float64) {
	remaining := math.Max(0, trade.FilledQuantity-trade.SoldQuantity)
	markValue := remaining * math.Max(0, bid)
	estimatedExitFee := markValue * math.Max(0, fee)
	trade.PnL = trade.SellQuote + markValue - trade.BuyQuote - trade.Fees - estimatedExitFee
}

func paperSignalAt(signal repo.PalisadeSignalState) time.Time {
	if !signal.SentAt.IsZero() {
		return signal.SentAt
	}
	return signal.UpdatedAt
}

func signalFor(signals []repo.PalisadeSignalState, symbol string) repo.PalisadeSignalState {
	for _, signal := range signals {
		if signal.Symbol == symbol {
			return signal
		}
	}
	return repo.PalisadeSignalState{}
}

func paperFillQuantity(remaining, topLevelQuantity float64) float64 {
	if remaining <= 0 {
		return 0
	}
	if topLevelQuantity <= 0 {
		return 0
	}
	return math.Min(remaining, topLevelQuantity)
}

func paperOpenedAt(trade repo.PaperTrade) time.Time {
	if trade.OpenedAt != nil {
		return *trade.OpenedAt
	}
	return trade.SignalAt
}

func isValidPaperOrder(symbol mexc.SymbolDetail, side order.Side, price, quantity float64) bool {
	return validateLimitOrder(symbol, side, price, quantity) == nil
}

func parseDecimalValue(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return 0, err
	}
	return parsed, nil
}
