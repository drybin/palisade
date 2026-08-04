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
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const (
	defaultSignalOrderUSDT = 10.0
	minSignalVolume24h     = 50000.0
	minSignalNetProfit     = 0.006
	maxSignalEntryRange    = 0.25
	minimumReboundPercent  = 0.001
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
	symbol, baseAsset                          string
	current, support, resistance, minExitPrice float64
	netProfit, spread, volume                  float64
	touchesSupport, touchesResistance, score   int
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
	klinesChecked := 0
	for _, snapshot := range snapshots {
		if snapshot.CollectedAt.IsZero() || time.Since(snapshot.CollectedAt) > 5*time.Minute {
			if debug {
				fmt.Printf("%s: снимок рынка устарел\n", snapshot.Symbol)
			}
			continue
		}
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
		klinesChecked++
		klines, err := u.api.GetKlinesPublic(ctx, snapshot.Symbol, enum.MINUTES_15, 100, nil)
		if err != nil {
			if debug {
				fmt.Printf("%s: свечи: %v\n", snapshot.Symbol, err)
			}
			continue
		}
		signal, ok := buildPalisadeSignal(snapshot, symbol, *klines, time.Now().UTC())
		if ok && isExecutablePalisadeSignal(signal, symbol) {
			candidates = append(candidates, signal)
		}
		if len(candidates) >= maxSignalCandidates {
			break
		}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	fmt.Printf("Snapshots загружено: %d\nСвечи запрошены для: %d пар\nОтобрано кандидатов: %d\n", len(snapshots), klinesChecked, len(candidates))
	for i, candidate := range candidates {
		fmt.Printf("%2d. %-16s цена=%-12s вход=%-12s цель=%-12s net=%.2f%% объём=%.0f спред=%.3f%% касания=S%d/R%d score=%d\n",
			i+1, candidate.symbol, formatPrice(candidate.current), formatPrice(candidate.support), formatPrice(candidate.resistance),
			candidate.netProfit*100, candidate.volume, candidate.spread*100, candidate.touchesSupport, candidate.touchesResistance, candidate.score)
	}
	sent := 0
	now := time.Now().UTC()
	for _, candidate := range candidates {
		lastSent, err := u.stateRepo.GetLastPalisadeSignal(ctx, candidate.symbol)
		if err != nil {
			return err
		}
		state := repo.PalisadeSignalState{
			Symbol:          candidate.symbol,
			StrategyVersion: paperStrategyVersion,
			SupportPrice:    candidate.support,
			EntryPrice:      candidate.current,
			TargetPrice:     candidate.resistance,
			MinExitPrice:    candidate.minExitPrice,
			NetProfit:       candidate.netProfit,
			Score:           candidate.score,
			Status:          "ACTIVE",
			ValidUntil:      now.Add(30 * time.Minute),
			UpdatedAt:       now,
		}
		if lastSent != nil && now.Sub(*lastSent) < signalCooldown {
			if err := u.stateRepo.SavePalisadeSignalState(ctx, state); err != nil {
				return wrap.Errorf("update signal state %s: %w", candidate.symbol, err)
			}
			continue
		}
		message := formatPalisadeSignal(candidate, now)
		if _, err := u.telegram.Send(message); err != nil {
			return wrap.Errorf("send signal %s: %w", candidate.symbol, err)
		}
		if err := u.stateRepo.SavePalisadeSignal(ctx, candidate.symbol, now, float64(candidate.score)); err != nil {
			return wrap.Errorf("save signal %s: %w", candidate.symbol, err)
		}
		if err := u.stateRepo.SavePalisadeSignalState(ctx, state); err != nil {
			return wrap.Errorf("save signal state %s: %w", candidate.symbol, err)
		}
		sent++
		if sent >= maxSignalsPerRun {
			break
		}
	}

	fmt.Printf("Отправлено сигналов: %d\n", sent)
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
	if snapshot.BidPrice < support {
		return palisadeSignal{}, false
	}
	current := snapshot.AskPrice
	if current < support || current > support+rangeValue*maxSignalEntryRange {
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
	lastClosed := rangeKlines[len(rangeKlines)-1]
	if lastClosed.Low > support+tolerance || lastClosed.Close < support*(1+minimumReboundPercent) {
		return palisadeSignal{}, false
	}
	makerFee := parseDecimal(symbol.MakerCommission)
	takerFee := parseDecimal(symbol.TakerCommission)
	fee := math.Max(makerFee, takerFee)
	entry := current
	spread := (snapshot.AskPrice - snapshot.BidPrice) / snapshot.BidPrice
	target, minExitPrice, ok := calculateDynamicTarget(entry, resistance, fee, spread)
	if !ok {
		return palisadeSignal{}, false
	}
	netProfit := target/entry - 1 - 2*fee - 0.001 - spread
	if netProfit < minSignalNetProfit {
		return palisadeSignal{}, false
	}
	score := int(netProfit*10000) + supportTouches*10 + resistanceTouches*10 - int(math.Abs((last-first)/first)*1000)
	return palisadeSignal{
		symbol: symbol.Symbol, baseAsset: symbol.BaseAsset, current: current, support: support,
		resistance: target, minExitPrice: minExitPrice, netProfit: netProfit, spread: spread,
		volume: snapshot.QuoteVolume24h, touchesSupport: supportTouches, touchesResistance: resistanceTouches, score: score,
	}, true
}

func isExecutablePalisadeSignal(signal palisadeSignal, symbol mexc.SymbolDetail) bool {
	step, err := swapLotStep(&symbol)
	if err != nil {
		return false
	}
	entry := roundPriceUp(signal.current, signalPriceStep(&symbol))
	quantity := swapRoundQtyDown(defaultSignalOrderUSDT/entry, step)
	return quantity > 0 && isValidPaperOrder(symbol, order.BUY, entry, quantity)
}

func calculateDynamicTarget(entry, resistance, fee, spread float64) (target, minExitPrice float64, ok bool) {
	if entry <= 0 || resistance <= entry || fee < 0 || spread < 0 {
		return 0, 0, false
	}
	target = resistance * 0.999
	minExitPrice = entry * (1 + 2*fee + 0.001 + spread + minSignalNetProfit)
	return target, minExitPrice, target >= minExitPrice
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
		"<b>📊 Палисада-кандидат v3</b> %s (%s)\n"+
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
