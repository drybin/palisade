package usecase

import (
	"math"
	"testing"
	"time"

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
