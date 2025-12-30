package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckPalisadeCoinList interface {
	Process(ctx context.Context, debug bool) error
}

type CheckPalisadeCoinList struct {
	checkerService *service.PalisadeCheckerService
	stateRepo      repo.IStateRepository
}

func NewCheckPalisadeCoinListUsecase(
	checkerService *service.PalisadeCheckerService,
	stateRepo repo.IStateRepository,
) *CheckPalisadeCoinList {
	return &CheckPalisadeCoinList{
		checkerService: checkerService,
		stateRepo:      stateRepo,
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
	totalProcessed := 0
	for _, coin := range data {
		currentIndex++
		if debug {
			fmt.Printf("[%d/%d] Обрабатываем монету: %s\n", currentIndex, totalCount, coin.Symbol)
		}

		// Используем сервис для проверки и обновления монеты
		result, err := u.checkerService.CheckAndUpdateCoin(ctx, service.CheckCoinParams{
			Symbol:               coin.Symbol,
			BaseAsset:            coin.BaseAsset,
			QuoteAsset:           coin.QuoteAsset,
			LastCheck:            coin.LastCheck,
			MinTimeSinceLastCheck: 180 * time.Minute,
			MaxVolatilityPercent: 5.0,
			Debug:                debug,
		})

		// Обрабатываем результат
		if err != nil {
			// Если ошибка критическая, прерываем обработку
			if result != nil && !result.Skipped {
				return err
			}
			// Иначе логируем и продолжаем
			if debug {
				fmt.Printf("Ошибка при обработке %s: %v\n", coin.Symbol, err)
			}
			continue
		}

		// Если монета пропущена, выводим причину в debug режиме
		if result.Skipped {
			if debug {
				fmt.Printf("Монета %s пропущена: %s\n", coin.Symbol, result.SkipReason)
			}
			continue
		}

		// Монета успешно обработана
		totalProcessed++
		time.Sleep(3 * time.Second)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Время начала обработки: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Время окончания обработки: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Общее время обработки: %v\n", elapsed)
	fmt.Printf("Обработано монет: %d из %d\n", totalProcessed, totalCount)
	return nil
}
