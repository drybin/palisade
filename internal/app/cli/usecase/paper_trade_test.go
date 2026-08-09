package usecase

import (
	"math"
	"testing"
	"time"

	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
)

func TestPaperFillQuantity_respectsTopLevelLiquidity(t *testing.T) {
	if got := paperFillQuantity(10, 3); got != 3 {
		t.Fatalf("expected 3, got %.8f", got)
	}
	if got := paperFillQuantity(10, 0); got != 0 {
		t.Fatalf("expected no fill when liquidity is unknown, got %.8f", got)
	}
}

func TestMarkPaperPnL_marksOpenPositionAtBidWithExitFee(t *testing.T) {
	trade := repo.PaperTrade{
		FilledQuantity: 10,
		SoldQuantity:   2,
		BuyQuote:       10,
		SellQuote:      2.2,
		Fees:           0.0122,
	}
	markPaperPnL(&trade, 0.95, 0.001)
	want := 2.2 + 8*0.95 - 10 - 0.0122 - 8*0.95*0.001
	if math.Abs(trade.PnL-want) > 1e-12 {
		t.Fatalf("expected marked P/L %.12f, got %.12f", want, trade.PnL)
	}
}

func TestMarkPaperPnL_closedTradeUsesRealizedAmounts(t *testing.T) {
	trade := repo.PaperTrade{
		FilledQuantity: 10,
		SoldQuantity:   10,
		BuyQuote:       10,
		SellQuote:      10.5,
		Fees:           0.0205,
	}
	markPaperPnL(&trade, 99, 0.001)
	want := 10.5 - 10 - 0.0205
	if math.Abs(trade.PnL-want) > 1e-12 {
		t.Fatalf("expected realized P/L %.12f, got %.12f", want, trade.PnL)
	}
}

func TestPaperSignalAt_prefersSentAt(t *testing.T) {
	sentAt := time.Date(2026, 8, 3, 1, 2, 3, 0, time.UTC)
	updatedAt := sentAt.Add(time.Minute)
	if got := paperSignalAt(repo.PalisadeSignalState{SentAt: sentAt, UpdatedAt: updatedAt}); !got.Equal(sentAt) {
		t.Fatalf("expected sent_at %s, got %s", sentAt, got)
	}
}

func TestPaperFillQuantity_doesNotExceedRemaining(t *testing.T) {
	if got := paperFillQuantity(2, 10); got != 2 {
		t.Fatalf("expected 2, got %.8f", got)
	}
}

func TestBuildPaperTrade_v5KeepsSupportAndAskEntry(t *testing.T) {
	trade, ok, err := buildPaperTrade(
		repo.PalisadeSignalState{
			Symbol: "TESTUSDT", SupportPrice: 100, EntryPrice: 100.1,
			TargetPrice: 102, MinExitPrice: 101, NetProfit: 0.01,
		},
		mexc.BookTicker{Symbol: "TESTUSDT", BidPrice: "100", AskPrice: "100.1"},
		mexc.SymbolDetail{
			Symbol: "TESTUSDT", Status: "1", IsSpotTradingAllowed: true,
			OrderTypes: []string{"LIMIT"}, QuotePrecision: 2, QuoteAmountPrecision: "1",
			BaseSizePrecision: "0.001", Filters: []mexc.SymbolFilter{{FilterType: "LOT_SIZE", StepSize: "0.001", MinQty: "0.001"}},
		},
		time.Now().UTC(),
	)
	if err != nil || !ok {
		t.Fatalf("expected valid v5 paper trade, ok=%v err=%v", ok, err)
	}
	if trade.EntryMode != "REBOUND_2CANDLE_V5" || math.Abs(trade.SupportPrice-100) > 1e-12 || math.Abs(trade.EntryPrice-100.1) > 1e-12 {
		t.Fatalf("unexpected v5 levels: mode=%s support=%.4f entry=%.4f", trade.EntryMode, trade.SupportPrice, trade.EntryPrice)
	}
	if math.Abs(trade.ExpectedNetProfit-0.01) > 1e-12 {
		t.Fatalf("expected net profit 0.01, got %.8f", trade.ExpectedNetProfit)
	}
}

func TestPaperExitReason_usesBreakEvenForV4AndLater(t *testing.T) {
	fee := 0.001
	buyPrice := 100.0
	breakEvenBid := paperBreakEvenBidPrice(buyPrice, fee)
	now := time.Now().UTC()

	currentTrade := repo.PaperTrade{StrategyVersion: paperStrategyVersion, BreakEvenArmed: true}
	if got := paperExitReason(currentTrade, now, breakEvenBid, 90, buyPrice, fee); got != "BREAKEVEN_STOP" {
		t.Fatalf("expected breakeven stop, got %q", got)
	}
	if trigger := paperBreakEvenTrigger(buyPrice, 101, fee); trigger <= breakEvenBid {
		t.Fatalf("expected trigger %.8f above break-even %.8f", trigger, breakEvenBid)
	}

	v4Trade := repo.PaperTrade{StrategyVersion: 4, BreakEvenArmed: true, SignalAt: now}
	if got := paperExitReason(v4Trade, now, breakEvenBid, 90, buyPrice, fee); got != "BREAKEVEN_STOP" {
		t.Fatalf("expected v4 break-even stop, got %q", got)
	}

	legacyTrade := repo.PaperTrade{StrategyVersion: 3, BreakEvenArmed: true, SignalAt: now}
	if got := paperExitReason(legacyTrade, now, breakEvenBid, 90, buyPrice, fee); got != "" {
		t.Fatalf("expected no v3 break-even stop, got %q", got)
	}
}

func TestPaperTrade_legacySupportFallsBackToEntry(t *testing.T) {
	trade := repo.PaperTrade{EntryPrice: 100}
	if trade.SupportPrice != 0 {
		t.Fatalf("expected legacy trade without stored support, got %.8f", trade.SupportPrice)
	}
	support := trade.SupportPrice
	if support <= 0 {
		support = trade.EntryPrice
	}
	if support != 100 {
		t.Fatalf("expected fallback support 100, got %.8f", support)
	}
}

func TestTrackPaperExcursion(t *testing.T) {
	trade := repo.PaperTrade{}
	trackPaperExcursion(&trade, 100)
	trackPaperExcursion(&trade, 102)
	trackPaperExcursion(&trade, 99)
	if trade.MaxBidPrice != 102 || trade.MinBidPrice != 99 {
		t.Fatalf("expected max/min 102/99, got %.4f/%.4f", trade.MaxBidPrice, trade.MinBidPrice)
	}
}
