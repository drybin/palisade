package repo

import (
	"context"
)

type IMexcRepository interface {
	GetBalance(ctx context.Context) error
}
