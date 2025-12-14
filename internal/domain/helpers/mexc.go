package helpers

import (
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/pkg/wrap"
)

// FindUSDTBalance находит баланс USDT в массиве балансов
func FindUSDTBalance(balances []mexc.Balance) (*mexc.Balance, error) {
	for i := range balances {
		if balances[i].Asset == "USDT" {
			return &balances[i], nil
		}
	}
	return nil, wrap.Errorf("USDT balance not found")
}
