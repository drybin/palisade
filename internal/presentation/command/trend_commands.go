package command

import (
	"context"
	"strings"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func trendFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "symbols",
			Usage: "comma-separated base assets (default TREND_WATCHLIST env)",
		},
		&cli.IntFlag{
			Name:  "days",
			Usage: "daily bars to fetch on backfill",
			Value: 100,
		},
		&cli.IntFlag{
			Name:  "sleep-ms",
			Usage: "pause between symbols (API rate limit)",
			Value: 200,
		},
		&cli.IntFlag{
			Name:  "sma-min",
			Value: 10,
		},
		&cli.IntFlag{
			Name:  "sma-max",
			Value: 100,
		},
		&cli.IntFlag{
			Name:  "sma-step",
			Value: 10,
		},
		&cli.Float64Flag{
			Name:  "retest-epsilon",
			Value: 0.1,
		},
		&cli.IntFlag{
			Name:  "retest-lookahead",
			Value: 60,
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "no Telegram, print only",
		},
	}
}

func parseTrendOpts(c *cli.Context) usecase.TrendOptions {
	opts := usecase.DefaultTrendOptions()
	if s := strings.TrimSpace(c.String("symbols")); s != "" {
		opts.Symbols = usecase.ParseTrendSymbols(s)
	}
	if c.IsSet("days") {
		opts.Days = c.Int("days")
	}
	if c.IsSet("sleep-ms") {
		opts.SleepMs = c.Int("sleep-ms")
	}
	if c.IsSet("sma-min") {
		opts.SMAMin = c.Int("sma-min")
	}
	if c.IsSet("sma-max") {
		opts.SMAMax = c.Int("sma-max")
	}
	if c.IsSet("sma-step") {
		opts.SMAStep = c.Int("sma-step")
	}
	if c.IsSet("retest-epsilon") {
		opts.RetestEpsilon = c.Float64("retest-epsilon")
	}
	if c.IsSet("retest-lookahead") {
		opts.RetestLookahead = c.Int("retest-lookahead")
	}
	opts.DryRun = c.Bool("dry-run")
	return opts
}

func NewBackfillTrendBarsCommand(uc usecase.IBackfillTrendBars) *cli.Command {
	return &cli.Command{
		Name:  "backfill-trend-bars",
		Usage: "Initial load of UTC daily closes for trend SMA (run once)",
		Flags: trendFlags(),
		Action: func(c *cli.Context) error {
			return uc.Process(context.Background(), parseTrendOpts(c))
		},
	}
}

func NewSyncTrendBarsCommand(uc usecase.ISyncTrendBars) *cli.Command {
	return &cli.Command{
		Name:  "sync-trend-bars",
		Usage: "Incremental sync: 1d (last 2) + 1m for today UTC",
		Flags: trendFlags(),
		Action: func(c *cli.Context) error {
			return uc.Process(context.Background(), parseTrendOpts(c))
		},
	}
}

func NewCheckTrendRetestCommand(uc usecase.ICheckTrendRetest) *cli.Command {
	return &cli.Command{
		Name:  "check-trend-retest",
		Usage: "Run retest FSM and send Telegram on entry signal",
		Flags: trendFlags(),
		Action: func(c *cli.Context) error {
			return uc.Process(context.Background(), parseTrendOpts(c))
		},
	}
}
