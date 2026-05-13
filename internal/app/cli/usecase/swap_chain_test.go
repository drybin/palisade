package usecase

import (
	"testing"

	"github.com/drybin/palisade/internal/domain/model/mexc"
)

func TestBuildSymbolDetailIndex(t *testing.T) {
	if BuildSymbolDetailIndex(nil) != nil {
		t.Fatal("nil info -> nil map")
	}
	info := &mexc.SymbolInfo{Symbols: []mexc.SymbolDetail{{Symbol: "ETHUSDT"}}}
	m := BuildSymbolDetailIndex(info)
	if m == nil || m["ETHUSDT"] == nil || m["ETHUSDT"].Symbol != "ETHUSDT" {
		t.Fatalf("index: %+v", m)
	}
}

func testSymbolWithLotAndTaker(sym, stepSize, takerCommission string) *mexc.SymbolDetail {
	return &mexc.SymbolDetail{
		Symbol:          sym,
		TakerCommission: takerCommission,
		Filters: []mexc.SymbolFilter{
			{FilterType: "LOT_SIZE", StepSize: stepSize},
		},
	}
}

func TestCalcSwapChainFromBook_metaVsContinuous(t *testing.T) {
	book := map[string]swapBookQuote{
		"ETHUSDT": {Bid: 2300, Ask: 2326},
		"BTCUSDT": {Bid: 78000, Ask: 78100},
		"ETHBTC":  {Bid: 0.0298, Ask: 0.03},
	}
	coinA := &mexc.SymbolDetail{BaseAsset: "ETH", Symbol: "ETHUSDT", QuoteAsset: "USDT"}
	coinB := &mexc.SymbolDetail{BaseAsset: "BTC", Symbol: "BTCUSDT", QuoteAsset: "USDT"}

	cont, ok := calcSwapChainFromBook(book, coinA, coinB, nil)
	if !ok {
		t.Fatal("continuous chain expected")
	}
	idx := map[string]*mexc.SymbolDetail{
		"ETHUSDT": testSymbolWithLotAndTaker("ETHUSDT", "0.00001", "0"),
		"BTCUSDT": testSymbolWithLotAndTaker("BTCUSDT", "0.000001", "0"),
		"ETHBTC":  testSymbolWithLotAndTaker("ETHBTC", "0.00001", "0"),
	}
	stepped, ok := calcSwapChainFromBook(book, coinA, coinB, &SwapChainMeta{Index: idx, InterBuffer: 0.999})
	if !ok {
		t.Fatal("stepped chain expected")
	}
	if stepped.profitPercent == cont.profitPercent {
		t.Fatalf("stepped profit %g should differ from continuous %g when lot steps apply", stepped.profitPercent, cont.profitPercent)
	}
}

func TestCalcSwapChainFromBook_feesLowerProfit(t *testing.T) {
	book := map[string]swapBookQuote{
		"ETHUSDT": {Bid: 2300, Ask: 2326},
		"BTCUSDT": {Bid: 78000, Ask: 78100},
		"ETHBTC":  {Bid: 0.0298, Ask: 0.03},
	}
	coinA := &mexc.SymbolDetail{BaseAsset: "ETH", Symbol: "ETHUSDT", QuoteAsset: "USDT"}
	coinB := &mexc.SymbolDetail{BaseAsset: "BTC", Symbol: "BTCUSDT", QuoteAsset: "USDT"}

	idxNoFee := map[string]*mexc.SymbolDetail{
		"ETHUSDT": testSymbolWithLotAndTaker("ETHUSDT", "0.00001", "0"),
		"BTCUSDT": testSymbolWithLotAndTaker("BTCUSDT", "0.000001", "0"),
		"ETHBTC":  testSymbolWithLotAndTaker("ETHBTC", "0.00001", "0"),
	}
	noFee, ok := calcSwapChainFromBook(book, coinA, coinB, &SwapChainMeta{Index: idxNoFee, InterBuffer: 0.999})
	if !ok {
		t.Fatal("expected ok")
	}

	idxFee := map[string]*mexc.SymbolDetail{
		"ETHUSDT": testSymbolWithLotAndTaker("ETHUSDT", "0.00001", "0.001"),
		"BTCUSDT": testSymbolWithLotAndTaker("BTCUSDT", "0.000001", "0.001"),
		"ETHBTC":  testSymbolWithLotAndTaker("ETHBTC", "0.00001", "0.001"),
	}
	withFee, ok := calcSwapChainFromBook(book, coinA, coinB, &SwapChainMeta{Index: idxFee, InterBuffer: 0.999})
	if !ok {
		t.Fatal("expected ok with fees")
	}
	if withFee.profitPercent >= noFee.profitPercent {
		t.Fatalf("with fees want lower profit: noFee=%g withFee=%g", noFee.profitPercent, withFee.profitPercent)
	}
}

func TestCalcSwapChainFromBook_requireRealismRejectsInvalidLotStep(t *testing.T) {
	book := map[string]swapBookQuote{
		"BDXUSDT": {Bid: 1.01, Ask: 1.02},
		"BTCUSDT": {Bid: 78000, Ask: 78100},
		"BDXBTC":  {Bid: 0.0000133, Ask: 0.0000134},
	}
	coinA := &mexc.SymbolDetail{BaseAsset: "BDX", Symbol: "BDXUSDT", QuoteAsset: "USDT"}
	coinB := &mexc.SymbolDetail{BaseAsset: "BTC", Symbol: "BTCUSDT", QuoteAsset: "USDT"}

	idx := map[string]*mexc.SymbolDetail{
		"BDXUSDT": testSymbolWithLotAndTaker("BDXUSDT", "0.1", "0"),
		"BTCUSDT": testSymbolWithLotAndTaker("BTCUSDT", "0.000001", "0"),
		"BDXBTC": {
			Symbol:             "BDXBTC",
			BaseSizePrecision:  "0",
			BaseAssetPrecision: 0,
		},
	}

	soft, ok := calcSwapChainFromBook(book, coinA, coinB, &SwapChainMeta{Index: idx, InterBuffer: 0.999})
	if !ok {
		t.Fatal("soft realism should fall back to continuous")
	}
	strict, ok := calcSwapChainFromBook(book, coinA, coinB, &SwapChainMeta{
		Index:          idx,
		InterBuffer:    0.999,
		RequireRealism: true,
	})
	if ok {
		t.Fatalf("strict realism should reject invalid lot step, got %+v", strict)
	}
	if soft.symbolAB == "" {
		t.Fatal("soft fallback should still return continuous route")
	}
}
