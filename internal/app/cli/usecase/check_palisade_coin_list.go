package usecase

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

type ICheckPalisadeCoinList interface {
	Process(ctx context.Context, debug bool) error
}

type CheckPalisadeCoinList struct {
	repo      *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewCheckPalisadeCoinListUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
) *CheckPalisadeCoinList {
	return &CheckPalisadeCoinList{
		repo:      repo,
		stateRepo: stateRepo,
	}
}

func (u *CheckPalisadeCoinList) Process(ctx context.Context, debug bool) error {
	startTime := time.Now()
	fmt.Println("check palisade")
	fmt.Printf("Время начала обработки: %s\n", startTime.Format("2006-01-02 15:04:05"))
	spotTradingAllowed := true
	isPalisade := true
	data, err := u.stateRepo.GetCoins(
		ctx,
		repo.GetCoinsParams{
			IsSpotTradingAllowed: &spotTradingAllowed,
			IsPalisade:           &isPalisade,
			Limit:                3000,
			Offset:               0,
		},
	)
	if err != nil {
		return wrap.Errorf("failed get coins to check: %w", err)
	}

	totalCount := len(data)
	currentIndex := 0
	for _, coin := range data {
		currentIndex++
		if debug {
			fmt.Printf("[%d/%d] Обрабатываем монету: %s\n", currentIndex, totalCount, coin.Symbol)
		}
		// проверяем последнюю дату проверки
		lastCheck := coin.LastCheck
		if time.Since(lastCheck) < 180*time.Minute {
			continue
		}

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

			// проверить есть ли в коде ошибки текст {"msg":"Invalid symbol.","code":-1121,"_extend":null} и если есть, то пропустить эту монету
			if strings.Contains(err.Error(), "Invalid symbol.") {
				fmt.Println("Invalid symbol", coin.Symbol)
				continue
			}

			return wrap.Errorf("failed to get klines: %w", err)
		}

		// Фильтруем свечи за последние 4 часа
		filteredKlines := filterKlinesLast4Hours(*klines)
		if len(filteredKlines) == 0 {
			if debug {
				fmt.Printf("Нет свечей за последние 4 часа для %s, пропускаем\n", coin.Symbol)
			}
			continue
		}

		// Анализируем флет с максимальной волатильностью 5%
		maxVolatilityPercent := 5.0
		flatAnalysis := filteredKlines.AnalyzeFlat(maxVolatilityPercent)

		// Выводим результаты анализа только при флаге debug
		if debug {
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
		}
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
		time.Sleep(3 * time.Second)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Время начала обработки: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Время окончания обработки: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Общее время обработки: %v\n", elapsed)
	return nil
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
