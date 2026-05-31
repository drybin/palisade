package usecase

import (
	"context"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

func utcDayFromOpenTimeMs(ms int64) time.Time {
	return service.UTCDayStart(time.UnixMilli(ms).UTC())
}

func persistDailyKlines(ctx context.Context, trendRepo repo.ITrendRepository, symbol string, klines mexc.Klines) error {
	for _, k := range klines {
		day := utcDayFromOpenTimeMs(k.OpenTime)
		if err := trendRepo.UpsertDailyBar(ctx, repo.MarketDailyBar{
			Symbol: symbol,
			DayUTC: day,
			Close:  k.Close,
		}); err != nil {
			return wrap.Errorf("upsert daily %s %s: %w", symbol, day.Format("2006-01-02"), err)
		}
	}
	return nil
}

func persistMinuteKlines(ctx context.Context, trendRepo repo.ITrendRepository, symbol string, klines mexc.Klines) error {
	for _, k := range klines {
		openTime := time.UnixMilli(k.OpenTime).UTC()
		if err := trendRepo.UpsertMinuteBar(ctx, repo.MarketMinuteBar{
			Symbol:   symbol,
			OpenTime: openTime,
			Open:     k.Open,
			High:     k.High,
			Low:      k.Low,
			Close:    k.Close,
		}); err != nil {
			return wrap.Errorf("upsert minute %s: %w", symbol, err)
		}
	}
	return nil
}

func fetchAndPersistDaily(
	ctx context.Context,
	mexcAPI *webapi.MexcWebapi,
	trendRepo repo.ITrendRepository,
	symbol string,
	limit int,
) (int, error) {
	klines, err := mexcAPI.GetKlinesPublic(ctx, symbol, enum.DAY_1, limit, nil)
	if err != nil {
		return 0, err
	}
	if err := persistDailyKlines(ctx, trendRepo, symbol, *klines); err != nil {
		return 0, err
	}
	return len(*klines), nil
}

func syncMinuteBarsToday(
	ctx context.Context,
	mexcAPI *webapi.MexcWebapi,
	trendRepo repo.ITrendRepository,
	symbol string,
) (int, error) {
	now := time.Now().UTC()
	dayStart := service.UTCDayStart(now)

	var startMs int64
	last, err := trendRepo.GetLastMinuteOpenTime(ctx, symbol)
	if err != nil {
		return 0, err
	}
	if last == nil || last.Before(dayStart) {
		startMs = dayStart.UnixMilli()
	} else {
		startMs = last.Add(time.Minute).UnixMilli()
	}

	total := 0
	for {
		klines, err := mexcAPI.GetKlinesPublic(ctx, symbol, enum.MINUTES_1, 1000, &startMs)
		if err != nil {
			return total, err
		}
		if len(*klines) == 0 {
			break
		}
		if err := persistMinuteKlines(ctx, trendRepo, symbol, *klines); err != nil {
			return total, err
		}
		total += len(*klines)
		lastK := (*klines)[len(*klines)-1]
		nextStart := lastK.CloseTime + 1
		if nextStart <= startMs {
			break
		}
		startMs = nextStart
		if len(*klines) < 1000 {
			break
		}
	}
	return total, nil
}
