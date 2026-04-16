package usecase

import (
	"testing"

	"github.com/drybin/palisade/internal/domain/model/mexc"
)

func TestSwapLotStep_SOLBTC_likeMEXC(t *testing.T) {
	s := &mexc.SymbolDetail{
		Symbol:              "SOLBTC",
		BaseSizePrecision:   "1",
		BaseAssetPrecision:  2,
	}
	step, err := swapLotStep(s)
	if err != nil {
		t.Fatal(err)
	}
	if step != 0.01 {
		t.Fatalf("want step 0.01, got %g", step)
	}
}

func TestSwapLotStep_fractionalBaseSizePrecision(t *testing.T) {
	s := &mexc.SymbolDetail{
		Symbol:              "BTCUSDT",
		BaseSizePrecision:   "0.000001",
		BaseAssetPrecision:  6,
	}
	step, err := swapLotStep(s)
	if err != nil {
		t.Fatal(err)
	}
	if step != 0.000001 {
		t.Fatalf("want step 0.000001, got %g", step)
	}
}
