package usecase

import (
	"math"
	"strings"
	"testing"

	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
)

func TestSignalPriceStep_prefersPriceFilter(t *testing.T) {
	symbol := mexc.SymbolDetail{
		QuotePrecision: 2,
		Filters:        []mexc.SymbolFilter{{FilterType: "PRICE_FILTER", StepSize: "0.0001"}},
	}
	if got := signalPriceStep(&symbol); got != 0.0001 {
		t.Fatalf("expected price filter step, got %.8f", got)
	}
}

func TestRoundPriceDown(t *testing.T) {
	if got := roundPriceDown(0.018449, 0.00001); got != 0.01844 {
		t.Fatalf("expected rounded price 0.01844, got %.8f", got)
	}
}

func TestRoundPriceUp(t *testing.T) {
	if got := roundPriceUp(0.018441, 0.00001); got != 0.01845 {
		t.Fatalf("expected 0.01845, got %.8f", got)
	}
}

func TestValidateLimitOrder_rejectsBelowMinimums(t *testing.T) {
	symbol := mexc.SymbolDetail{
		Symbol:               "TESTUSDT",
		Status:               "1",
		IsSpotTradingAllowed: true,
		OrderTypes:           []string{"LIMIT"},
		QuoteAmountPrecision: "1",
		BaseSizePrecision:    "0.1",
		Filters: []mexc.SymbolFilter{{
			FilterType: "LOT_SIZE",
			StepSize:   "0.1",
			MinQty:     "1",
		}},
	}
	if err := validateLimitOrder(symbol, order.BUY, 1, 0.1); err == nil || !strings.Contains(err.Error(), "minQty") {
		t.Fatalf("expected minQty error, got %v", err)
	}
}

func TestValidateLimitOrder_rejectsWrongSide(t *testing.T) {
	symbol := mexc.SymbolDetail{Symbol: "TESTUSDT", Status: "1", IsSpotTradingAllowed: true, OrderTypes: []string{"LIMIT"}, TradeSideType: 2}
	if err := validateLimitOrder(symbol, order.SELL, 1, 1); err == nil {
		t.Fatal("expected SELL-only restriction error")
	}
}

func TestValidateLimitOrder_acceptsAlignedOrder(t *testing.T) {
	symbol := mexc.SymbolDetail{
		Symbol:               "TESTUSDT",
		Status:               "1",
		IsSpotTradingAllowed: true,
		OrderTypes:           []string{"LIMIT"},
		QuoteAmountPrecision: "1",
		Filters:              []mexc.SymbolFilter{{FilterType: "LOT_SIZE", StepSize: "0.1", MinQty: "1", MaxQty: "10"}},
	}
	if err := validateLimitOrder(symbol, order.BUY, 1, 1); err != nil {
		t.Fatalf("expected valid order, got %v", err)
	}
}

func TestNewSignalClientOrderID_isUniqueAndUsesSignalPrefix(t *testing.T) {
	first, err := newSignalClientOrderID(order.BUY)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSignalClientOrderID(order.BUY)
	if err != nil {
		t.Fatal(err)
	}
	sell, err := newSignalClientOrderID(order.SELL)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("expected unique client order ids, got %q", first)
	}
	if !strings.HasPrefix(first, "Signal_B_") || !strings.HasPrefix(sell, "Signal_S_") {
		t.Fatalf("unexpected client order ids: BUY=%q SELL=%q", first, sell)
	}
	if len(first) > 32 || len(sell) > 32 {
		t.Fatalf("client order id is too long: BUY=%d SELL=%d", len(first), len(sell))
	}
}

func TestSummarizeSellIntents_aggregatesAttemptsAndKeepsInitialPlan(t *testing.T) {
	progress, err := summarizeSellIntents([]repo.OrderIntent{
		{ID: 1, Side: order.BUY.String(), Quantity: 10, ExecutedQuantity: 10, CumulativeQuoteQty: 5},
		{ID: 2, Side: order.SELL.String(), Quantity: 9.9, ExecutedQuantity: 3, CumulativeQuoteQty: 1.8},
		{ID: 3, Side: order.SELL.String(), Quantity: 6.9, ExecutedQuantity: 6.9, CumulativeQuoteQty: 4.3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.planned != 9.9 || progress.executed != 9.9 || math.Abs(progress.quote-6.1) > 1e-12 {
		t.Fatalf("unexpected progress: %+v", progress)
	}
	if progress.remaining() != 0 {
		t.Fatalf("expected no remaining quantity, got %.8f", progress.remaining())
	}
}

func TestSummarizeSellIntents_rejectsOversell(t *testing.T) {
	_, err := summarizeSellIntents([]repo.OrderIntent{
		{ID: 1, Side: order.SELL.String(), Quantity: 5, ExecutedQuantity: 3},
		{ID: 2, Side: order.SELL.String(), Quantity: 3, ExecutedQuantity: 3},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds planned position") {
		t.Fatalf("expected oversell error, got %v", err)
	}
}
