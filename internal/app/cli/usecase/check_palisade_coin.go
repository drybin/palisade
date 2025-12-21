package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckPalisadeCoin interface {
	Process(ctx context.Context, symbol string, percentile float64) error
}

type CheckPalisadeCoin struct {
	repo      *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewCheckPalisadeCoinUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
) *CheckPalisadeCoin {
	return &CheckPalisadeCoin{
		repo:      repo,
		stateRepo: stateRepo,
	}
}

func (u *CheckPalisadeCoin) Process(ctx context.Context, symbol string, percentile float64) error {
	fmt.Printf("check palisade for coin: %s\n", symbol)

	// Получаем информацию о монете из базы данных
	coin, err := u.stateRepo.GetCoinInfo(ctx, symbol)
	if err != nil {
		return wrap.Errorf("failed to get coin info: %w", err)
	}

	if coin == nil {
		return wrap.Errorf("coin %s not found in database", symbol)
	}

	//// Проверяем последнюю дату проверки
	//lastCheck := coin.LastCheck
	//if time.Since(lastCheck) < 180*time.Minute {
	//	fmt.Printf("Coin %s was checked recently (less than 3 hours ago), skipping\n", symbol)
	//	return nil
	//}

	//fmt.Println("Last check:", lastCheck)

	// Получаем свечи
	klines, err := u.repo.GetKlines(
		model.PairWithLevels{
			Pair: model.Pair{
				CoinFirst:  model.Coin{Name: coin.BaseAsset, SymbolId: coin.Symbol},
				CoinSecond: model.Coin{Name: coin.QuoteAsset, SymbolId: coin.Symbol},
			},
		},
		enum.MINUTES_15,
	)
	if err != nil {
		// Проверяем, есть ли ошибка "Invalid symbol"
		if strings.Contains(err.Error(), "Invalid symbol.") {
			fmt.Printf("Invalid symbol: %s\n", coin.Symbol)
			return nil
		}

		return wrap.Errorf("failed to get klines: %w", err)
	}

	// Фильтруем свечи за последние 4 часа
	var filteredKlines = filterKlinesLast4Hours(*klines)
	if len(filteredKlines) == 0 {
		fmt.Printf("Нет свечей за последние 4 часа для %s\n", coin.Symbol)
		return nil
	}

	// Применяем фильтрацию по перцентилю, если он указан
	if percentile >= 1 && percentile <= 100 {
		originalCount := len(filteredKlines)
		filteredKlines = filteredKlines.FilterByPercentile(percentile)
		fmt.Printf("Применена фильтрация по перцентилю %.0f: %d -> %d свечей\n", percentile, originalCount, len(filteredKlines))
	}

	// Анализируем флет с максимальной волатильностью 5%
	maxVolatilityPercent := 5.0
	flatAnalysis := filteredKlines.AnalyzeFlat(maxVolatilityPercent)

	// Выводим результаты анализа
	fmt.Printf("\n=== Анализ для %s ===\n", coin.Symbol)
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

	// Обновляем статус isPalisade в базе данных
	err = u.stateRepo.UpdateIsPalisade(ctx, coin.Symbol, flatAnalysis.IsFlat)
	if err != nil {
		return wrap.Errorf("failed to update isPalisade for coin %s: %w", coin.Symbol, err)
	}

	if flatAnalysis.IsFlat {
		err = u.stateRepo.UpdatePalisadeParams(ctx, coin.Symbol, flatAnalysis.Support, flatAnalysis.Resistance, flatAnalysis.Range, flatAnalysis.RangePercent, flatAnalysis.AvgPrice, flatAnalysis.Volatility, flatAnalysis.MaxDrawdown, flatAnalysis.MaxRise)
		if err != nil {
			return wrap.Errorf("failed to update palisade params for coin %s: %w", coin.Symbol, err)
		}
	}

	return nil
}
