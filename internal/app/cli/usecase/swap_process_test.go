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

func TestSwapLotStep_prefersLOT_SIZE_whenBaseFieldsWrong(t *testing.T) {
	// MEXC sometimes yields baseSizePrecision "1" with baseAssetPrecision 0; legacy logic would use step 1.0.
	s := &mexc.SymbolDetail{
		Symbol:              "SOLBTC",
		BaseSizePrecision:   "1",
		BaseAssetPrecision:  0,
		Filters: []mexc.SymbolFilter{
			{FilterType: "LOT_SIZE", StepSize: "0.01", MinQty: "0.01"},
		},
	}
	step, err := swapLotStep(s)
	if err != nil {
		t.Fatal(err)
	}
	if step != 0.01 {
		t.Fatalf("want LOT_SIZE step 0.01, got %g", step)
	}
}
