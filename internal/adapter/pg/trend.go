package registry

import (
	"context"
	"errors"
	"time"

	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
	palisade_database "github.com/drybin/palisade/sqlc/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TrendRepository struct {
	Postgree *pgx.Conn
}

func NewTrendRepository(pg *pgx.Conn) TrendRepository {
	return TrendRepository{Postgree: pg}
}

func (r TrendRepository) UpsertDailyBar(ctx context.Context, bar repo.MarketDailyBar) error {
	db := palisade_database.New(r.Postgree)
	return db.UpsertMarketDailyBar(ctx, palisade_database.UpsertMarketDailyBarParams{
		Symbol: bar.Symbol,
		DayUtc: timeToPgDate(bar.DayUTC),
		Close:  bar.Close,
	})
}

func (r TrendRepository) CountDailyBars(ctx context.Context, symbol string) (int64, error) {
	db := palisade_database.New(r.Postgree)
	return db.CountMarketDailyBars(ctx, symbol)
}

func (r TrendRepository) ListDailyBars(ctx context.Context, symbol string) ([]repo.MarketDailyBar, error) {
	db := palisade_database.New(r.Postgree)
	rows, err := db.ListMarketDailyBars(ctx, symbol)
	if err != nil {
		return nil, wrap.Errorf("list daily bars: %w", err)
	}
	out := make([]repo.MarketDailyBar, 0, len(rows))
	for _, row := range rows {
		out = append(out, repo.MarketDailyBar{
			Symbol: row.Symbol,
			DayUTC: pgDateToUTC(row.DayUtc),
			Close:  row.Close,
		})
	}
	return out, nil
}

func (r TrendRepository) UpsertMinuteBar(ctx context.Context, bar repo.MarketMinuteBar) error {
	db := palisade_database.New(r.Postgree)
	return db.UpsertMarketMinuteBar(ctx, palisade_database.UpsertMarketMinuteBarParams{
		Symbol:   bar.Symbol,
		OpenTime: bar.OpenTime,
		Open:     bar.Open,
		High:     bar.High,
		Low:      bar.Low,
		Close:    bar.Close,
	})
}

func (r TrendRepository) GetLastMinuteOpenTime(ctx context.Context, symbol string) (*time.Time, error) {
	db := palisade_database.New(r.Postgree)
	t, err := db.GetLastMinuteBarOpenTime(ctx, symbol)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("last minute open time: %w", err)
	}
	return &t, nil
}

func (r TrendRepository) ListMinuteBarsFrom(ctx context.Context, symbol string, from time.Time) ([]repo.MarketMinuteBar, error) {
	db := palisade_database.New(r.Postgree)
	rows, err := db.ListMarketMinuteBarsFrom(ctx, palisade_database.ListMarketMinuteBarsFromParams{
		Symbol:   symbol,
		OpenTime: from,
	})
	if err != nil {
		return nil, wrap.Errorf("list minute bars: %w", err)
	}
	out := make([]repo.MarketMinuteBar, 0, len(rows))
	for _, row := range rows {
		out = append(out, repo.MarketMinuteBar{
			Symbol:   row.Symbol,
			OpenTime: row.OpenTime,
			Open:     row.Open,
			High:     row.High,
			Low:      row.Low,
			Close:    row.Close,
		})
	}
	return out, nil
}

func (r TrendRepository) GetRetestState(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time) (*repo.TrendRetestState, error) {
	db := palisade_database.New(r.Postgree)
	row, err := db.GetTrendRetestState(ctx, palisade_database.GetTrendRetestStateParams{
		Symbol:    symbol,
		SmaPeriod: smaPeriod,
		DayUtc:    timeToPgDate(dayUTC),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("get retest state: %w", err)
	}
	return &repo.TrendRetestState{
		Symbol:                row.Symbol,
		SmaPeriod:             row.SmaPeriod,
		DayUTC:                pgDateToUTC(row.DayUtc),
		WaitRetest:            row.WaitRetest,
		RetestUntil:           row.RetestUntil,
		LastProcessedOpenTime: row.LastProcessedOpenTime,
	}, nil
}

func (r TrendRepository) SaveRetestState(ctx context.Context, state repo.TrendRetestState) error {
	db := palisade_database.New(r.Postgree)
	return db.UpsertTrendRetestState(ctx, palisade_database.UpsertTrendRetestStateParams{
		Symbol:                state.Symbol,
		SmaPeriod:             state.SmaPeriod,
		DayUtc:                timeToPgDate(state.DayUTC),
		WaitRetest:            state.WaitRetest,
		RetestUntil:           state.RetestUntil,
		LastProcessedOpenTime: state.LastProcessedOpenTime,
	})
}

func (r TrendRepository) WasSignalSent(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time, kind string) (bool, error) {
	db := palisade_database.New(r.Postgree)
	return db.TrendSignalWasSent(ctx, palisade_database.TrendSignalWasSentParams{
		Symbol:     symbol,
		SmaPeriod:  smaPeriod,
		DayUtc:     timeToPgDate(dayUTC),
		SignalKind: kind,
	})
}

func (r TrendRepository) RecordSignalSent(ctx context.Context, symbol string, smaPeriod int, dayUTC time.Time, kind string) error {
	db := palisade_database.New(r.Postgree)
	return db.InsertTrendSignalSent(ctx, palisade_database.InsertTrendSignalSentParams{
		Symbol:     symbol,
		SmaPeriod:  smaPeriod,
		DayUtc:     timeToPgDate(dayUTC),
		SignalKind: kind,
	})
}

func timeToPgDate(t time.Time) pgtype.Date {
	t = t.UTC()
	var d pgtype.Date
	_ = d.Scan(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
	return d
}

func pgDateToUTC(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
}
