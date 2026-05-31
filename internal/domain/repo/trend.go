package repo

import (
	"context"
	"time"
)

const TrendSignalRetestEntry = "retest_entry"

type MarketDailyBar struct {
	Symbol string
	DayUTC time.Time
	Close  float64
}

type MarketMinuteBar struct {
	Symbol   string
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

type TrendRetestState struct {
	Symbol                string
	SmaPeriod             int
	DayUTC                time.Time
	WaitRetest            bool
	RetestUntil           *time.Time
	LastProcessedOpenTime *time.Time
}

type ITrendRepository interface {
	UpsertDailyBar(ctx context.Context, bar MarketDailyBar) error
	CountDailyBars(ctx context.Context, symbol string) (int64, error)
	ListDailyBars(ctx context.Context, symbol string) ([]MarketDailyBar, error)

	UpsertMinuteBar(ctx context.Context, bar MarketMinuteBar) error
	GetLastMinuteOpenTime(ctx context.Context, symbol string) (*time.Time, error)
	ListMinuteBarsFrom(ctx context.Context, symbol string, from time.Time) ([]MarketMinuteBar, error)

	GetRetestState(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time) (*TrendRetestState, error)
	SaveRetestState(ctx context.Context, state TrendRetestState) error

	WasSignalSent(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time, kind string) (bool, error)
	RecordSignalSent(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time, kind string) error
}
