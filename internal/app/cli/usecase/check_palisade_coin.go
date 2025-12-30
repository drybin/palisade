package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckPalisadeCoin interface {
	Process(ctx context.Context, symbol string, percentile float64) error
}

type CheckPalisadeCoin struct {
	checkerService *service.PalisadeCheckerService
	stateRepo      repo.IStateRepository
}

func NewCheckPalisadeCoinUsecase(
	checkerService *service.PalisadeCheckerService,
	stateRepo repo.IStateRepository,
) *CheckPalisadeCoin {
	return &CheckPalisadeCoin{
		checkerService: checkerService,
		stateRepo:      stateRepo,
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

	// Используем сервис для проверки и обновления монеты
	result, err := u.checkerService.CheckAndUpdateCoin(ctx, service.CheckCoinParams{
		Symbol:                coin.Symbol,
		BaseAsset:             coin.BaseAsset,
		QuoteAsset:            coin.QuoteAsset,
		LastCheck:             time.Time{}, // Не проверяем время последней проверки (всегда обрабатываем)
		MinTimeSinceLastCheck: 0,           // Отключаем проверку времени
		MaxVolatilityPercent:  5.0,
		Percentile:            percentile,
		Debug:                 true, // Всегда показываем результаты для одной монеты
	})

	// Обрабатываем результат
	if err != nil {
		return err
	}

	// Если монета пропущена, выводим причину
	if result.Skipped {
		fmt.Printf("Монета %s пропущена: %s\n", coin.Symbol, result.SkipReason)
		return nil
	}

	fmt.Println("✅ Монета успешно проверена и обновлена")
	return nil
}
