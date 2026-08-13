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
			price = 103.0
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
	klines[len(klines)-2].Open = 99.95
	klines[len(klines)-2].Low = 99.90
	klines[len(klines)-2].High = 100.25
	klines[len(klines)-2].Close = 100.15
	klines[len(klines)-1].Open = 100.16
	klines[len(klines)-1].Low = 100.15
	klines[len(klines)-1].High = 100.35
	klines[len(klines)-1].Close = 100.30

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
	if signal.current != snapshot.AskPrice {
		t.Fatalf("expected current ask %.4f, got %.4f", snapshot.AskPrice, signal.current)
	}
	if signal.support >= signal.entry || signal.entry >= signal.current {
		t.Fatalf("expected pullback entry between support and current price, support=%.4f entry=%.4f current=%.4f", signal.support, signal.entry, signal.current)
	}
	if signal.touchesSupport < 2 || signal.touchesResistance < 2 {
		t.Fatalf("expected repeated touches, got support=%d resistance=%d", signal.touchesSupport, signal.touchesResistance)
	}
}

func TestIsBTCMarketSafe_rejectsSharpThirtyMinuteDecline(t *testing.T) {
	now := time.Now().UTC()
	klines := mexc.Klines{
		{Close: 100, CloseTime: now.Add(-31 * time.Minute).UnixMilli()},
		{Close: 99.8, CloseTime: now.Add(-16 * time.Minute).UnixMilli()},
		{Close: 99.4, CloseTime: now.Add(-time.Minute).UnixMilli()},
	}
	if isBTCMarketSafe(klines, now) {
		t.Fatal("expected a 0.6% BTC decline to block new signals")
	}
	klines[2].Close = 99.6
	if !isBTCMarketSafe(klines, now) {
		t.Fatal("expected a 0.4% BTC decline to allow new signals")
	}
}

func TestBuildPalisadeSignal_requiresLastClosedCandleRebound(t *testing.T) {
	now := time.Now().UTC()
	klines := make(mexc.Klines, 0, 96)
	for i := 0; i < 96; i++ {
		price := 100.0
		if i%8 == 4 {
			price = 102.0
		}
		open := now.Add(-time.Duration(97-i) * 15 * time.Minute)
		klines = append(klines, mexc.Kline{
			OpenTime: open.UnixMilli(), Open: price, High: price + 0.1,
			Low: price - 0.1, Close: price,
			CloseTime: open.Add(15 * time.Minute).UnixMilli(),
		})
	}
	klines[len(klines)-1].Low = 100.3
	klines[len(klines)-1].High = 100.5
	klines[len(klines)-1].Open = 100.4
	klines[len(klines)-1].Close = 100.3

	snapshot := repo.MarketSnapshot{Symbol: "NOREBOUNDUSDT", BidPrice: 100.2, AskPrice: 100.3, QuoteVolume24h: 100000}
	symbol := mexc.SymbolDetail{Symbol: "NOREBOUNDUSDT", MakerCommission: "0", TakerCommission: "0"}
	if _, ok := buildPalisadeSignal(snapshot, symbol, klines, now); ok {
		t.Fatal("expected a range without a bullish confirmation candle to be rejected")
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

func TestBuildPalisadeSignal_rejectsPriceBelowSupport(t *testing.T) {
	now := time.Now().UTC()
	klines := make(mexc.Klines, 0, 96)
	for i := 0; i < 96; i++ {
		price := 100.0
		if i%8 == 4 {
			price = 102.0
		}
		open := now.Add(-time.Duration(96-i) * 15 * time.Minute)
		klines = append(klines, mexc.Kline{
			OpenTime: open.UnixMilli(), Open: price, High: price + 0.1,
			Low: price - 0.1, Close: price,
			CloseTime: open.Add(15 * time.Minute).UnixMilli(),
		})
	}

	snapshot := repo.MarketSnapshot{Symbol: "BELOWUSDT", BidPrice: 98, AskPrice: 98.1, QuoteVolume24h: 100000}
	symbol := mexc.SymbolDetail{Symbol: "BELOWUSDT", MakerCommission: "0", TakerCommission: "0"}
	if _, ok := buildPalisadeSignal(snapshot, symbol, klines, now); ok {
		t.Fatal("expected price below support to be rejected")
	}
}

func TestCalculateDynamicTarget_rejectsTargetBelowMinimumExit(t *testing.T) {
	target, minExit, ok := calculateDynamicTarget(100, 101, 0.001, 0.003)
	if ok {
		t.Fatal("expected target below minimum exit to be rejected")
	}
	if target <= 0 || minExit <= target {
		t.Fatalf("expected positive minimum exit above target, got target=%.4f minExit=%.4f", target, minExit)
	}
}

func TestCalculateDynamicTarget_acceptsTargetWithMinimumProfit(t *testing.T) {
	target, minExit, ok := calculateDynamicTarget(100, 103, 0.0005, 0.001)
	if !ok {
		t.Fatal("expected target with minimum profit to be accepted")
	}
	if target < minExit {
		t.Fatalf("expected target >= minimum exit, got target=%.4f minExit=%.4f", target, minExit)
	}
	if target != 101.2 {
		t.Fatalf("expected target at 40%% of range, got %.4f", target)
	}
}
