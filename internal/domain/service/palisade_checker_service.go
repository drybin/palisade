package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type PalisadeCheckerService struct {
	mexcRepo  *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewPalisadeCheckerService(
	mexcRepo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
) *PalisadeCheckerService {
	return &PalisadeCheckerService{
		mexcRepo:  mexcRepo,
		stateRepo: stateRepo,
	}
}

// CoinCheckResult представляет результат проверки монеты
type CoinCheckResult struct {
	Symbol       string
	IsFlat       bool
	FlatAnalysis *mexc.FlatAnalysisResult
	Error        error
	Skipped      bool
	SkipReason   string
}

// CheckCoinParams параметры для проверки монеты
type CheckCoinParams struct {
	Symbol                string
	BaseAsset             string
	QuoteAsset            string
	LastCheck             time.Time
	MinTimeSinceLastCheck time.Duration // минимальное время с последней проверки (по умолчанию 180 минут)
	MaxVolatilityPercent  float64       // максимальная волатильность для флета (по умолчанию 5%)
	Percentile            float64       // перцентиль для фильтрации свечей (0 = не использовать)
	Debug                 bool
}

// CheckAndUpdateCoin проверяет монету и обновляет её параметры в базе данных
func (s *PalisadeCheckerService) CheckAndUpdateCoin(ctx context.Context, params CheckCoinParams) (*CoinCheckResult, error) {
	result := &CoinCheckResult{
		Symbol: params.Symbol,
	}

	// Устанавливаем значения по умолчанию
	if params.MinTimeSinceLastCheck == 0 {
		params.MinTimeSinceLastCheck = 180 * time.Minute
	}
	if params.MaxVolatilityPercent == 0 {
		params.MaxVolatilityPercent = 5.0
	}

	// Проверяем последнюю дату проверки
	if time.Since(params.LastCheck) < params.MinTimeSinceLastCheck {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("последняя проверка была менее %v назад", params.MinTimeSinceLastCheck)
		return result, nil
	}

	// Получаем свечи
	klines, err := s.mexcRepo.GetKlines(
		model.PairWithLevels{
			Pair: model.Pair{
				CoinFirst:  model.Coin{Name: params.BaseAsset, SymbolId: params.Symbol},
				CoinSecond: model.Coin{Name: params.QuoteAsset, SymbolId: params.Symbol},
			},
		},
		enum.MINUTES_15,
	)
	if err != nil {
		// Проверяем на Invalid symbol
		if strings.Contains(err.Error(), "Invalid symbol.") {
			result.Skipped = true
			result.SkipReason = "Invalid symbol"
			result.Error = err
			return result, nil
		}
		result.Error = err
		return result, wrap.Errorf("failed to get klines for %s: %w", params.Symbol, err)
	}

	// Фильтруем свечи за последние 4 часа
	filteredKlines := filterKlinesLast4Hours(*klines)
	if len(filteredKlines) == 0 {
		result.Skipped = true
		result.SkipReason = "нет свечей за последние 4 часа"
		return result, nil
	}

	// Применяем фильтрацию по перцентилю, если указано
	if params.Percentile >= 1 && params.Percentile <= 100 {
		originalCount := len(filteredKlines)
		filteredKlines = filteredKlines.FilterByPercentile(params.Percentile)
		if params.Debug {
			fmt.Printf("Применена фильтрация по перцентилю %.0f: %d -> %d свечей\n", params.Percentile, originalCount, len(filteredKlines))
		}
	}

	// Анализируем флет
	flatAnalysis := filteredKlines.AnalyzeFlat(params.MaxVolatilityPercent)
	result.IsFlat = flatAnalysis.IsFlat
	result.FlatAnalysis = &flatAnalysis

	// Выводим результаты анализа только при флаге debug
	if params.Debug {
		fmt.Printf("\n=== Анализ для %s ===\n", params.Symbol)
		fmt.Printf("Количество свечей (всего/за последние 4 часа): %d/%d\n", len(*klines), len(filteredKlines))
		fmt.Printf("Во флете: %v\n", flatAnalysis.IsFlat)
		fmt.Printf("Нижняя граница (Support): %.8f\n", flatAnalysis.Support)
		fmt.Printf("Верхняя граница (Resistance): %.8f\n", flatAnalysis.Resistance)
		fmt.Printf("Диапазон: %.8f\n", flatAnalysis.Range)
		fmt.Printf("Диапазон в процентах: %.2f%%\n", flatAnalysis.RangePercent)
		fmt.Printf("Средняя цена: %.8f\n", flatAnalysis.AvgPrice)
		fmt.Printf("Волатильность: %.2f%%\n", flatAnalysis.Volatility)
		fmt.Printf("Максимальная просадка: %.2f%%\n", flatAnalysis.MaxDrawdown)
		fmt.Printf("Максимальный рост: %.2f%%\n", flatAnalysis.MaxRise)
		fmt.Printf("================================\n\n")
	}

	// Обновляем статус isPalisade в базе данных
	err = s.stateRepo.UpdateIsPalisade(ctx, params.Symbol, flatAnalysis.IsFlat)
	if err != nil {
		return result, wrap.Errorf("failed to update isPalisade for coin %s: %w", params.Symbol, err)
	}

	// Если монета во флете, обновляем параметры палисады
	if flatAnalysis.IsFlat {
		err = s.stateRepo.UpdatePalisadeParams(
			ctx,
			params.Symbol,
			flatAnalysis.Support,
			flatAnalysis.Resistance,
			flatAnalysis.Range,
			flatAnalysis.RangePercent,
			flatAnalysis.AvgPrice,
			flatAnalysis.Volatility,
			flatAnalysis.MaxDrawdown,
			flatAnalysis.MaxRise,
		)
		if err != nil {
			return result, wrap.Errorf("failed to update palisade params for coin %s: %w", params.Symbol, err)
		}
	}

	return result, nil
}

// filterKlinesLast4Hours фильтрует свечи, оставляя только те, которые открылись в последние 4 часа
func filterKlinesLast4Hours(klines mexc.Klines) mexc.Klines {
	if len(klines) == 0 {
		return klines
	}

	// Вычисляем время 4 часа назад от текущего момента (в миллисекундах)
	now := time.Now()
	fourHoursAgo := now.Add(-4 * time.Hour)
	cutoffTime := fourHoursAgo.Unix() * 1000 // Unix timestamp в миллисекундах

	filtered := make(mexc.Klines, 0)
	for _, kline := range klines {
		// OpenTime в миллисекундах, сравниваем с cutoffTime
		if kline.OpenTime >= cutoffTime {
			filtered = append(filtered, kline)
		}
	}

	return filtered
}
