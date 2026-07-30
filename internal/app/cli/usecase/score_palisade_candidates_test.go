package usecase

import (
	"testing"
	"time"

	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
)

func TestBuildPalisadeSignal_requiresStableRangeAndNetProfit(t *testing.T) {
	now := time.Now().UTC()
	klines := make(mexc.Klines, 0, 96)
	for i := 0; i < 96; i++ {
		price := 100.0
		if i%24 == 0 {
			price = 100.2
		}
		if i%8 == 4 {
			price = 102.0
		}
		open := now.Add(-time.Duration(96-i) * 15 * time.Minute)
		klines = append(klines, mexc.Kline{
			OpenTime:  open.UnixMilli(),
			Open:      price,
			High:      price + 0.1,
			Low:       price - 0.1,
			Close:     price,
			CloseTime: open.Add(15 * time.Minute).UnixMilli(),
		})
	}

	snapshot := repo.MarketSnapshot{
		Symbol:         "TESTUSDT",
		BidPrice:       100.2,
		AskPrice:       100.3,
		QuoteVolume24h: 100000,
	}
	symbol := mexc.SymbolDetail{Symbol: "TESTUSDT", BaseAsset: "TEST", MakerCommission: "0.0005", TakerCommission: "0.0005"}

	signal, ok := buildPalisadeSignal(snapshot, symbol, klines, now)
	if !ok {
		t.Fatal("expected stable range to produce a signal")
	}
	if signal.netProfit < minSignalNetProfit {
		t.Fatalf("expected net profit >= %.4f, got %.4f", minSignalNetProfit, signal.netProfit)
	}
	if signal.touchesSupport < 2 || signal.touchesResistance < 2 {
		t.Fatalf("expected repeated touches, got support=%d resistance=%d", signal.touchesSupport, signal.touchesResistance)
	}
}

func TestBuildPalisadeSignal_rejectsTrend(t *testing.T) {
	now := time.Now().UTC()
	klines := make(mexc.Klines, 0, 96)
	for i := 0; i < 96; i++ {
		price := 100 + float64(i)*0.1
		open := now.Add(-time.Duration(96-i) * 15 * time.Minute)
		klines = append(klines, mexc.Kline{
			OpenTime: open.UnixMilli(),
			High:     price + 0.1, Low: price - 0.1, Close: price,
			CloseTime: open.Add(15 * time.Minute).UnixMilli(),
		})
	}

	snapshot := repo.MarketSnapshot{Symbol: "TRENDUSDT", BidPrice: 100, AskPrice: 100.1, QuoteVolume24h: 100000}
	symbol := mexc.SymbolDetail{Symbol: "TRENDUSDT", MakerCommission: "0", TakerCommission: "0"}
	if _, ok := buildPalisadeSignal(snapshot, symbol, klines, now); ok {
		t.Fatal("expected trending market to be rejected")
	}
}
