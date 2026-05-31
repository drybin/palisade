package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ISyncTrendBars interface {
	Process(ctx context.Context, opts TrendOptions) error
}

type SyncTrendBars struct {
	mexcAPI   *webapi.MexcWebapi
	trendRepo repo.ITrendRepository
}

func NewSyncTrendBarsUsecase(mexcAPI *webapi.MexcWebapi, trendRepo repo.ITrendRepository) *SyncTrendBars {
	return &SyncTrendBars{mexcAPI: mexcAPI, trendRepo: trendRepo}
}

func (u *SyncTrendBars) Process(ctx context.Context, opts TrendOptions) error {
	if len(opts.Symbols) == 0 {
		return wrap.Errorf("no symbols in watchlist")
	}
	sleep := time.Duration(opts.SleepMs) * time.Millisecond
	if sleep <= 0 {
		sleep = defaultTrendSleepMs * time.Millisecond
	}

	fmt.Printf("sync-trend-bars: %d symbols\n", len(opts.Symbols))
	for i, symbol := range opts.Symbols {
		dailyN, err := fetchAndPersistDaily(ctx, u.mexcAPI, u.trendRepo, symbol, 2)
		if err != nil {
			fmt.Printf("[%d/%d] %s daily ERROR: %v\n", i+1, len(opts.Symbols), symbol, err)
		} else {
			fmt.Printf("[%d/%d] %s daily: %d bars\n", i+1, len(opts.Symbols), symbol, dailyN)
		}
		minN, err := syncMinuteBarsToday(ctx, u.mexcAPI, u.trendRepo, symbol)
		if err != nil {
			fmt.Printf("[%d/%d] %s minute ERROR: %v\n", i+1, len(opts.Symbols), symbol, err)
		} else {
			fmt.Printf("[%d/%d] %s minute today: +%d bars\n", i+1, len(opts.Symbols), symbol, minN)
		}
		time.Sleep(sleep)
	}
	fmt.Println("sync done")
	return nil
}
