package usecase

import (
	"os"
	"strconv"
	"strings"

	"github.com/drybin/palisade/pkg/env"
)

const (
	defaultTrendWatchlist = "BTC,ETH,BNB,XRP,SOL,TRX,HYPE,DOGE,ZEC,XLM,ADA,XMR,LINK,BCH,TON,LTC,SUI,SHIB,DOT,OKB,ASTER," +
		"AVAX,UNI,ATOM,NEAR,APT,ARB,OP,POL,ICP,FIL,HBAR,ETC,INJ,RENDER," +
		"TAO,SEI,STX,WLD,PEPE"
	defaultTrendDays      = 100
	defaultTrendSleepMs   = 200
	defaultTrendSMAMin    = 10
	defaultTrendSMAMax    = 100
	defaultTrendSMAStep   = 10
	defaultRetestEpsilon  = 0.1
	defaultRetestLook     = 60
)

type TrendOptions struct {
	Symbols        []string
	Days           int
	SleepMs        int
	SMAMin         int
	SMAMax         int
	SMAStep        int
	RetestEpsilon  float64
	RetestLookahead int
	DryRun         bool
}

func DefaultTrendOptions() TrendOptions {
	return TrendOptions{
		Symbols:         ParseTrendSymbols(env.GetString("TREND_WATCHLIST", defaultTrendWatchlist)),
		Days:            env.GetInt("TREND_BACKFILL_DAYS", defaultTrendDays),
		SleepMs:         env.GetInt("TREND_API_SLEEP_MS", defaultTrendSleepMs),
		SMAMin:          env.GetInt("TREND_SMA_MIN", defaultTrendSMAMin),
		SMAMax:          env.GetInt("TREND_SMA_MAX", defaultTrendSMAMax),
		SMAStep:         env.GetInt("TREND_SMA_STEP", defaultTrendSMAStep),
		RetestEpsilon:   trendEnvFloat("TREND_RETEST_EPSILON", defaultRetestEpsilon),
		RetestLookahead: env.GetInt("TREND_RETEST_LOOKAHEAD", defaultRetestLook),
	}
}

func ParseTrendSymbols(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, strings.ToUpper(p)+"USDT")
	}
	return out
}

func trendEnvFloat(name string, defaultVal float64) float64 {
	v := os.Getenv(name)
	if strings.TrimSpace(v) == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func SMAPeriods(opts TrendOptions) []int {
	var periods []int
	for p := opts.SMAMin; p <= opts.SMAMax; p += opts.SMAStep {
		if p > 0 {
			periods = append(periods, p)
		}
	}
	return periods
}
