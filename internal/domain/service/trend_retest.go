package service

import (
	"sort"
	"time"

	"github.com/drybin/palisade/internal/domain/repo"
)

// MinuteBar — одна минутная свеча для retest FSM.
type MinuteBar struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

// RetestFSMState — состояние автомата retest на UTC-день.
type RetestFSMState struct {
	WaitRetest            bool
	RetestUntil           time.Time
	LastProcessedOpenTime time.Time
}

// RetestSignal — сигнал входа по retest.
type RetestSignal struct {
	EntryPrice float64
	SMA        float64
	TouchMin   float64
	TouchMax   float64
	OpenTime   time.Time
}

// UTCDayStart обрезает время до полуночи UTC.
func UTCDayStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// DailyClosesMap строит map UTC-дня -> close из дневных баров.
func DailyClosesMap(bars []repo.MarketDailyBar) map[time.Time]float64 {
	out := make(map[time.Time]float64, len(bars))
	for _, b := range bars {
		out[UTCDayStart(b.DayUTC)] = b.Close
	}
	return out
}

// DailySMA — среднее daily close за period дней, заканчивая endDay (включительно).
func DailySMA(closes map[time.Time]float64, endDay time.Time, period int) (float64, bool) {
	if period <= 0 {
		return 0, false
	}
	endDay = UTCDayStart(endDay)
	days := sortedDaysUpTo(closes, endDay)
	if len(days) < period {
		return 0, false
	}
	slice := days[len(days)-period:]
	var sum float64
	for _, d := range slice {
		sum += closes[d]
	}
	return sum / float64(period), true
}

// TrendBull — вчера close > SMA(period) на вчера; равенство → не bull.
func TrendBull(closes map[time.Time]float64, yesterday time.Time, period int) (bool, float64, float64) {
	yesterday = UTCDayStart(yesterday)
	closePrev, ok := closes[yesterday]
	if !ok {
		return false, 0, 0
	}
	smaPrev, ok := DailySMA(closes, yesterday, period)
	if !ok {
		return false, closePrev, 0
	}
	if closePrev <= smaPrev {
		return false, closePrev, smaPrev
	}
	return true, closePrev, smaPrev
}

// SMATodayMinute — SMA на минутном сигнале: dailySMA[вчера] для period.
func SMATodayMinute(closes map[time.Time]float64, today time.Time, period int) (float64, bool) {
	yesterday := UTCDayStart(today).AddDate(0, 0, -1)
	return DailySMA(closes, yesterday, period)
}

// ProcessRetestFSM прогоняет новые свечи через автомат retest (§5.7 документа).
func ProcessRetestFSM(
	candles []MinuteBar,
	sma float64,
	epsilonPct float64,
	lookahead int,
	st RetestFSMState,
) (RetestFSMState, *RetestSignal) {
	if lookahead <= 0 {
		lookahead = 60
	}
	touchMin := sma * (1 - epsilonPct/100)
	touchMax := sma * (1 + epsilonPct/100)

	for i := 0; i < len(candles); i++ {
		c := candles[i]
		if !st.LastProcessedOpenTime.IsZero() && !c.OpenTime.After(st.LastProcessedOpenTime) {
			continue
		}

		if st.WaitRetest {
			if !st.RetestUntil.IsZero() && c.OpenTime.After(st.RetestUntil) {
				st.WaitRetest = false
				st.RetestUntil = time.Time{}
				st.LastProcessedOpenTime = c.OpenTime
				continue
			}
			touched := c.Low <= touchMax && c.High >= touchMin
			if touched {
				st.WaitRetest = false
				st.RetestUntil = time.Time{}
				st.LastProcessedOpenTime = c.OpenTime
				return st, &RetestSignal{
					EntryPrice: c.Close,
					SMA:        sma,
					TouchMin:   touchMin,
					TouchMax:   touchMax,
					OpenTime:   c.OpenTime,
				}
			}
			st.LastProcessedOpenTime = c.OpenTime
			continue
		}

		if i == 0 {
			st.LastProcessedOpenTime = c.OpenTime
			continue
		}
		prev := candles[i-1]
		if prev.Close <= sma && c.Close > sma {
			st.WaitRetest = true
			st.RetestUntil = c.OpenTime.Add(time.Duration(lookahead) * time.Minute)
			st.LastProcessedOpenTime = c.OpenTime
			continue
		}
		st.LastProcessedOpenTime = c.OpenTime
	}
	return st, nil
}

func sortedDaysUpTo(closes map[time.Time]float64, endDay time.Time) []time.Time {
	endDay = UTCDayStart(endDay)
	var days []time.Time
	for d := range closes {
		if !d.After(endDay) {
			days = append(days, d)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days
}
