package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckTrendRetest interface {
	Process(ctx context.Context, opts TrendOptions) error
}

type CheckTrendRetest struct {
	mexcAPI     *webapi.MexcWebapi
	trendRepo   repo.ITrendRepository
	telegramAPI *webapi.TelegramWebapi
}

func NewCheckTrendRetestUsecase(
	mexcAPI *webapi.MexcWebapi,
	trendRepo repo.ITrendRepository,
	telegramAPI *webapi.TelegramWebapi,
) *CheckTrendRetest {
	return &CheckTrendRetest{mexcAPI: mexcAPI, trendRepo: trendRepo, telegramAPI: telegramAPI}
}

func (u *CheckTrendRetest) Process(ctx context.Context, opts TrendOptions) error {
	if len(opts.Symbols) == 0 {
		return wrap.Errorf("no symbols in watchlist")
	}
	periods := SMAPeriods(opts)
	if len(periods) == 0 {
		return wrap.Errorf("no SMA periods configured")
	}

	today := service.UTCDayStart(time.Now().UTC())
	yesterday := today.AddDate(0, 0, -1)
	dayStart := today

	sleep := time.Duration(opts.SleepMs) * time.Millisecond
	signals := 0

	for _, symbol := range opts.Symbols {
		if _, err := fetchAndPersistDaily(ctx, u.mexcAPI, u.trendRepo, symbol, 2); err != nil {
			fmt.Printf("%s sync daily: %v\n", symbol, err)
		}
		if _, err := syncMinuteBarsToday(ctx, u.mexcAPI, u.trendRepo, symbol); err != nil {
			fmt.Printf("%s sync minutes: %v\n", symbol, err)
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}

		daily, err := u.trendRepo.ListDailyBars(ctx, symbol)
		if err != nil {
			return wrap.Errorf("daily bars %s: %w", symbol, err)
		}
		closes := service.DailyClosesMap(daily)

		minutes, err := u.trendRepo.ListMinuteBarsFrom(ctx, symbol, dayStart)
		if err != nil {
			return wrap.Errorf("minute bars %s: %w", symbol, err)
		}
		candles := make([]service.MinuteBar, len(minutes))
		for i, m := range minutes {
			candles[i] = service.MinuteBar{
				OpenTime: m.OpenTime,
				Open:     m.Open,
				High:     m.High,
				Low:      m.Low,
				Close:    m.Close,
			}
		}

		for _, period := range periods {
			bull, _, _ := service.TrendBull(closes, yesterday, period)
			if !bull {
				_ = u.trendRepo.SaveRetestState(ctx, repo.TrendRetestState{
					Symbol: symbol, SmaPeriod: period, DayUTC: today,
					WaitRetest: false,
				})
				continue
			}

			sent, err := u.trendRepo.WasSignalSent(ctx, symbol, period, today, repo.TrendSignalRetestEntry)
			if err != nil {
				return err
			}
			if sent {
				continue
			}

			sma, ok := service.SMATodayMinute(closes, today, period)
			if !ok || sma <= 0 {
				continue
			}

			st := service.RetestFSMState{}
			dbSt, err := u.trendRepo.GetRetestState(ctx, symbol, period, today)
			if err != nil {
				return err
			}
			if dbSt != nil {
				st.WaitRetest = dbSt.WaitRetest
				if dbSt.RetestUntil != nil {
					st.RetestUntil = *dbSt.RetestUntil
				}
				if dbSt.LastProcessedOpenTime != nil {
					st.LastProcessedOpenTime = *dbSt.LastProcessedOpenTime
				}
			}

			newSt, sig := service.ProcessRetestFSM(candles, sma, opts.RetestEpsilon, opts.RetestLookahead, st)
			_ = u.trendRepo.SaveRetestState(ctx, repo.TrendRetestState{
				Symbol:                symbol,
				SmaPeriod:             period,
				DayUTC:                today,
				WaitRetest:            newSt.WaitRetest,
				RetestUntil:           timePtr(newSt.RetestUntil),
				LastProcessedOpenTime: timePtr(newSt.LastProcessedOpenTime),
			})

			if sig == nil {
				continue
			}

			msg := fmt.Sprintf(
				"🟢 <b>Retest LONG</b> %s | SMA %d | %s UTC\nSMA %.8g (ε %.2f%%) | entry close %.8g\n%s",
				symbol, period, sig.OpenTime.UTC().Format("2006-01-02 15:04"),
				sig.SMA, opts.RetestEpsilon, sig.EntryPrice, sig.OpenTime.UTC().Format(time.RFC3339),
			)
			if opts.DryRun {
				fmt.Println("[dry-run]", msg)
			} else {
				if u.telegramAPI == nil || !u.telegramAPI.Configured() {
					fmt.Println("[no TG]", msg)
				} else {
					if _, err := u.telegramAPI.Send(msg); err != nil {
						fmt.Printf("TG error %s SMA%d: %v\n", symbol, period, err)
						continue
					}
				}
				if err := u.trendRepo.RecordSignalSent(ctx, symbol, period, today, repo.TrendSignalRetestEntry); err != nil {
					return wrap.Errorf("record signal: %w", err)
				}
			}
			signals++
		}
	}

	fmt.Printf("check-trend-retest done, signals: %d\n", signals)
	return nil
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
