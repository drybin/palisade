package usecase

import (
	"context"
	"fmt"

	"github.com/drybin/palisade/internal/domain/repo"
)

const tradingLockKey = "palisade:spot-trading"

func acquireTradingLock(ctx context.Context, stateRepo repo.IStateRepository) (func(), bool, error) {
	return acquireNamedLock(ctx, stateRepo, tradingLockKey)
}

func acquireNamedLock(ctx context.Context, stateRepo repo.IStateRepository, key string) (func(), bool, error) {
	acquired, err := stateRepo.TryAcquireTradingLock(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		fmt.Printf("Lock %s уже занят, этот запуск пропущен\n", key)
		return nil, false, nil
	}
	return func() {
		if err := stateRepo.ReleaseTradingLock(context.Background(), key); err != nil {
			fmt.Printf("Освобождение trading lock: %v\n", err)
		}
	}, true, nil
}
