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

const paperLockKey = "palisade:paper-trading"

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

	openTrades, err := u.stateRepo.ListOpenPaperTrades(ctx)
	if err != nil {
		return err
	}
	openBySymbol := make(map[string]repo.PaperTrade, len(openTrades))
	for _, trade := range openTrades {
		openBySymbol[trade.Symbol] = trade
	}

	for i := range openTrades {
		trade := openTrades[i]
		if err := u.processPaperTrade(ctx, &trade, signalFor(signals, trade.Symbol), bookBySymbol[trade.Symbol], bySymbol[trade.Symbol], time.Now().UTC()); err != nil {
			return err
		}
		if debug {
			fmt.Printf("paper %s: status=%s filled=%.8f sold=%.8f pnl=%.8f\n", trade.Symbol, trade.Status, trade.FilledQuantity, trade.SoldQuantity, trade.PnL)
		}
	}

	created := 0
	for _, signal := range signals {
		if _, exists := openBySymbol[signal.Symbol]; exists {
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
		if err := u.processPaperTrade(ctx, createdTrade, signal, book, symbol, time.Now().UTC()); err != nil {
			return err
		}
	}

	stats, err := u.stateRepo.GetPaperTradeStats(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Paper trading: открытых=%d, новых=%d, всего=%d, закрыто=%d, P/L=%.8f USDT, win=%d, loss=%d\n",
		stats.Open, created, stats.Total, stats.Closed, stats.TotalPnL, stats.Wins, stats.Losses)
	return nil
}

func buildPaperTrade(signal repo.PalisadeSignalState, book mexc.BookTicker, symbol mexc.SymbolDetail, now time.Time) (repo.PaperTrade, bool, error) {
	bid, ask, err := parseBook(book)
	if err != nil || ask <= bid || ask <= 0 {
		return repo.PaperTrade{}, false, nil
	}
	entry := roundPriceDown(signal.EntryPrice, signalPriceStep(&symbol))
	step, err := swapLotStep(&symbol)
	if err != nil {
		return repo.PaperTrade{}, false, err
	}
	quantity := swapRoundQtyDown(signalOrderQuoteUSDT/entry, step)
	if quantity <= 0 || !isValidPaperOrder(symbol, order.BUY, entry, quantity) {
		return repo.PaperTrade{}, false, nil
	}
	return repo.PaperTrade{
		Symbol:       signal.Symbol,
		SignalAt:     signal.UpdatedAt,
		Status:       "BUY_PENDING",
		EntryPrice:   entry,
		TargetPrice:  roundPriceDown(signal.TargetPrice, signalPriceStep(&symbol)),
		MinExitPrice: roundPriceDown(signal.MinExitPrice, signalPriceStep(&symbol)),
		Quantity:     quantity,
		LastPrice:    (bid + ask) / 2,
		UpdatedAt:    now,
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

	if signal.Symbol == trade.Symbol && signal.TargetPrice >= signal.MinExitPrice {
		if trade.Status == "BUY_PENDING" || signal.TargetPrice > trade.TargetPrice {
			trade.TargetPrice = roundPriceDown(signal.TargetPrice, signalPriceStep(&symbol))
			trade.MinExitPrice = roundPriceDown(signal.MinExitPrice, signalPriceStep(&symbol))
		}
	}

	if trade.Status == "BUY_PENDING" {
		if bid < trade.EntryPrice*(1-supportBreakPercent) || now.Sub(trade.SignalAt) > signalBuyTimeout {
			if trade.FilledQuantity == 0 {
				trade.Status = "CANCELED"
				trade.ExitReason = "BUY_NOT_FILLED"
				return u.stateRepo.UpdatePaperTrade(ctx, *trade)
			}
			trade.Status = "POSITION_OPEN"
		}
		if trade.Status == "BUY_PENDING" && ask <= trade.EntryPrice {
			remaining := trade.Quantity - trade.FilledQuantity
			fillQty := paperFillQuantity(remaining, askQty)
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
			return u.stateRepo.UpdatePaperTrade(ctx, *trade)
		}
		buyPrice := trade.BuyQuote / trade.FilledQuantity
		reason := emergencyReason(now, paperOpenedAt(*trade), bid, trade.EntryPrice, buyPrice)
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
			trade.PnL = trade.SellQuote - trade.BuyQuote - trade.Fees
			trade.Status = "CLOSED"
			closed := now
			trade.ClosedAt = &closed
		}
	}
	trade.PnL = trade.SellQuote - trade.BuyQuote - trade.Fees
	trade.UpdatedAt = now
	return u.stateRepo.UpdatePaperTrade(ctx, *trade)
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
		return remaining
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
