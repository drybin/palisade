package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type IBackfillTrendBars interface {
	Process(ctx context.Context, opts TrendOptions) error
}

type BackfillTrendBars struct {
	mexcAPI   *webapi.MexcWebapi
	trendRepo repo.ITrendRepository
}

func NewBackfillTrendBarsUsecase(mexcAPI *webapi.MexcWebapi, trendRepo repo.ITrendRepository) *BackfillTrendBars {
	return &BackfillTrendBars{mexcAPI: mexcAPI, trendRepo: trendRepo}
}

func (u *BackfillTrendBars) Process(ctx context.Context, opts TrendOptions) error {
	if len(opts.Symbols) == 0 {
		return wrap.Errorf("no symbols in watchlist")
	}
	if opts.Days <= 0 {
		opts.Days = defaultTrendDays
	}
	sleep := time.Duration(opts.SleepMs) * time.Millisecond
	if sleep <= 0 {
		sleep = defaultTrendSleepMs * time.Millisecond
	}

	fmt.Printf("backfill-trend-bars: %d symbols, %d daily candles each\n", len(opts.Symbols), opts.Days)

	for i, symbol := range opts.Symbols {
		n, err := fetchAndPersistDaily(ctx, u.mexcAPI, u.trendRepo, symbol, opts.Days)
		if err != nil {
			fmt.Printf("[%d/%d] %s ERROR: %v\n", i+1, len(opts.Symbols), symbol, err)
			time.Sleep(sleep)
			continue
		}
		cnt, _ := u.trendRepo.CountDailyBars(ctx, symbol)
		fmt.Printf("[%d/%d] %s: saved %d klines, total in db: %d\n", i+1, len(opts.Symbols), symbol, n, cnt)
		time.Sleep(sleep)
	}
	fmt.Println("backfill done")
	return nil
}
