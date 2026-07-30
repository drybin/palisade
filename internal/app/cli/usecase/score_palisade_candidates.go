package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const (
	defaultSignalOrderUSDT = 10.0
	minSignalVolume24h     = 50000.0
	minSignalNetProfit     = 0.006
	signalCooldown         = 60 * time.Minute
	maxSignalCandidates    = 100
	maxSignalsPerRun       = 3
)

type IScorePalisadeCandidates interface {
	Process(context.Context, bool) error
}

type ScorePalisadeCandidates struct {
	api       *webapi.MexcWebapi
	stateRepo repo.IStateRepository
	telegram  *webapi.TelegramWebapi
}

func NewScorePalisadeCandidatesUsecase(api *webapi.MexcWebapi, stateRepo repo.IStateRepository, telegram *webapi.TelegramWebapi) *ScorePalisadeCandidates {
	return &ScorePalisadeCandidates{api: api, stateRepo: stateRepo, telegram: telegram}
}

type palisadeSignal struct {
	symbol, baseAsset                                       string
	current, support, resistance, netProfit, spread, volume float64
	touchesSupport, touchesResistance, score                int
}

func (u *ScorePalisadeCandidates) Process(ctx context.Context, debug bool) error {
	if !u.telegram.Configured() {
		return wrap.Errorf("telegram is not configured")
	}
	snapshots, err := u.stateRepo.ListMarketSnapshots(ctx)
	if err != nil {
		return err
	}
	info, err := u.api.GetExchangeInfoAll(ctx)
	if err != nil {
		return wrap.Errorf("get exchange info: %w", err)
	}
	bySymbol := make(map[string]mexc.SymbolDetail, len(info.Symbols))
	for _, symbol := range info.Symbols {
		if symbol.IsSpotTradingAllowed && symbol.QuoteAsset == "USDT" {
			bySymbol[symbol.Symbol] = symbol
		}
	}

	candidates := make([]palisadeSignal, 0, maxSignalCandidates)
	for _, snapshot := range snapshots {
		if snapshot.QuoteVolume24h < minSignalVolume24h || snapshot.BidPrice <= 0 || snapshot.AskPrice <= snapshot.BidPrice {
			continue
		}
		symbol, ok := bySymbol[snapshot.Symbol]
		if !ok {
			continue
		}
		spread := (snapshot.AskPrice - snapshot.BidPrice) / snapshot.BidPrice
		if spread > 0.003 {
			continue
		}
		klines, err := u.api.GetKlinesPublic(ctx, snapshot.Symbol, enum.MINUTES_15, 100, nil)
		if err != nil {
			if debug {
				fmt.Printf("%s: свечи: %v\n", snapshot.Symbol, err)
			}
			continue
		}
		signal, ok := buildPalisadeSignal(snapshot, symbol, *klines, time.Now().UTC())
		if ok {
			candidates = append(candidates, signal)
		}
		if len(candidates) >= maxSignalCandidates {
			break
		}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	sent := 0
	now := time.Now().UTC()
	for _, candidate := range candidates {
		lastSent, err := u.stateRepo.GetLastPalisadeSignal(ctx, candidate.symbol)
		if err != nil {
			return err
		}
		if lastSent != nil && now.Sub(*lastSent) < signalCooldown {
			continue
		}
		message := formatPalisadeSignal(candidate, now)
		if _, err := u.telegram.Send(message); err != nil {
			return wrap.Errorf("send signal %s: %w", candidate.symbol, err)
		}
		if err := u.stateRepo.SavePalisadeSignal(ctx, candidate.symbol, now, float64(candidate.score)); err != nil {
			return wrap.Errorf("save signal %s: %w", candidate.symbol, err)
		}
		sent++
		if sent >= maxSignalsPerRun {
			break
		}
	}

	fmt.Printf("Проверено кандидатов: %d, отправлено сигналов: %d\n", len(candidates), sent)
	return nil
}

func buildPalisadeSignal(snapshot repo.MarketSnapshot, symbol mexc.SymbolDetail, klines mexc.Klines, now time.Time) (palisadeSignal, bool) {
	closed := make(mexc.Klines, 0, len(klines))
	for _, kline := range klines {
		if kline.CloseTime > 0 && time.UnixMilli(kline.CloseTime).UTC().Before(now) {
			closed = append(closed, kline)
		}
	}
	if len(closed) < 48 {
		return palisadeSignal{}, false
	}
	if len(closed) > 96 {
		closed = closed[len(closed)-96:]
	}
	rangeKlines := closed
	if len(rangeKlines) > 48 {
		rangeKlines = rangeKlines[len(rangeKlines)-48:]
	}

	lows := make([]float64, 0, len(rangeKlines))
	highs := make([]float64, 0, len(rangeKlines))
	for _, kline := range rangeKlines {
		if kline.Low > 0 && kline.High >= kline.Low {
			lows = append(lows, kline.Low)
			highs = append(highs, kline.High)
		}
	}
	if len(lows) < 48 {
		return palisadeSignal{}, false
	}
	sort.Float64s(lows)
	sort.Float64s(highs)
	support := percentileValue(lows, 0.10)
	resistance := percentileValue(highs, 0.90)
	if support <= 0 || resistance <= support {
		return palisadeSignal{}, false
	}
	rangeValue := resistance - support
	current := (snapshot.BidPrice + snapshot.AskPrice) / 2
	if current > support+rangeValue*0.30 {
		return palisadeSignal{}, false
	}
	first := closed[0].Close
	last := closed[len(closed)-1].Close
	if first <= 0 || math.Abs((last-first)/first) > 0.015 {
		return palisadeSignal{}, false
	}
	tolerance := math.Max(rangeValue*0.12, support*0.0015)
	supportTouches, resistanceTouches := 0, 0
	for _, kline := range rangeKlines {
		if math.Abs(kline.Low-support) <= tolerance {
			supportTouches++
		}
		if math.Abs(kline.High-resistance) <= tolerance {
			resistanceTouches++
		}
	}
	if supportTouches < 2 || resistanceTouches < 2 {
		return palisadeSignal{}, false
	}
	makerFee := parseDecimal(symbol.MakerCommission)
	takerFee := parseDecimal(symbol.TakerCommission)
	fee := math.Max(makerFee, takerFee)
	entry := support
	target := resistance * 0.999
	netProfit := target/entry - 1 - 2*fee - 0.001 - (snapshot.AskPrice-snapshot.BidPrice)/snapshot.BidPrice
	if netProfit < minSignalNetProfit {
		return palisadeSignal{}, false
	}
	score := int(netProfit*10000) + supportTouches*10 + resistanceTouches*10 - int(math.Abs((last-first)/first)*1000)
	return palisadeSignal{
		symbol: symbol.Symbol, baseAsset: symbol.BaseAsset, current: current, support: entry,
		resistance: target, netProfit: netProfit, spread: (snapshot.AskPrice - snapshot.BidPrice) / snapshot.BidPrice,
		volume: snapshot.QuoteVolume24h, touchesSupport: supportTouches, touchesResistance: resistanceTouches, score: score,
	}, true
}

func percentileValue(values []float64, percentile float64) float64 {
	index := int(math.Round(float64(len(values)-1) * percentile))
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func parseDecimal(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	if parsed > 1 {
		return parsed / 100
	}
	return parsed
}

func formatPalisadeSignal(signal palisadeSignal, now time.Time) string {
	return fmt.Sprintf(
		"<b>📊 Палисада-кандидат</b> %s (%s)\n"+
			"Цена: %s\nВход: %s | Цель: %s\n"+
			"Net-прибыль: %.2f%%\nКасания: S=%d, R=%d\n"+
			"Расчётный объём: %.2f USDT\n"+
			"24h оборот: %.0f USDT | Спред: %.3f%%\n"+
			"Score: %d\nАктуально до: %s\n"+
			"<i>Dry-run: ордера не размещаются</i>",
		signal.symbol, signal.baseAsset, formatPrice(signal.current), formatPrice(signal.support), formatPrice(signal.resistance),
		signal.netProfit*100, signal.touchesSupport, signal.touchesResistance, defaultSignalOrderUSDT, signal.volume, signal.spread*100,
		signal.score, now.Add(30*time.Minute).Format("2006-01-02 15:04:05 MST"),
	)
}

func formatPrice(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", value), "0"), ".")
}
