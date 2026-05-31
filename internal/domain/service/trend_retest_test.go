package service

import (
	"testing"
	"time"

	"github.com/drybin/palisade/internal/domain/repo"
)

func TestTrendBullAndDailySMA(t *testing.T) {
	day := func(s string) time.Time {
		tm, _ := time.Parse("2006-01-02", s)
		return UTCDayStart(tm)
	}
	closes := map[time.Time]float64{
		day("2026-05-20"): 100,
		day("2026-05-21"): 102,
		day("2026-05-22"): 104,
		day("2026-05-23"): 106,
		day("2026-05-24"): 108,
		day("2026-05-25"): 110,
		day("2026-05-26"): 112,
		day("2026-05-27"): 114,
		day("2026-05-28"): 116,
		day("2026-05-29"): 118,
	}
	bull, closePrev, smaPrev := TrendBull(closes, day("2026-05-29"), 10)
	if !bull {
		t.Fatalf("expected bull, close=%v sma=%v", closePrev, smaPrev)
	}
	bars := make([]repo.MarketDailyBar, 0, len(closes))
	for d, c := range closes {
		bars = append(bars, repo.MarketDailyBar{DayUTC: d, Close: c})
	}
	_ = DailyClosesMap(bars)
}

func TestProcessRetestFSM_touch(t *testing.T) {
	sma := 100.0
	base := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	candles := []MinuteBar{
		{OpenTime: base, Close: 99},
		{OpenTime: base.Add(time.Minute), Close: 101}, // breakout
		{OpenTime: base.Add(2 * time.Minute), Low: 99.5, High: 100.5, Close: 100.2}, // retest touch
	}
	st, sig := ProcessRetestFSM(candles, sma, 0.1, 60, RetestFSMState{})
	if sig == nil {
		t.Fatal("expected retest signal")
	}
	if sig.EntryPrice != 100.2 {
		t.Fatalf("entry %v", sig.EntryPrice)
	}
	if st.WaitRetest {
		t.Fatal("waitRetest should be false after signal")
	}
}
