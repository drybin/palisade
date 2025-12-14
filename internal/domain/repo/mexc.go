package repo

import (
	"context"

	"github.com/drybin/palisade/internal/domain/model/mexc"
)

type IMexcRepository interface {
	GetBalance(ctx context.Context) (*mexc.AccountInfo, error)
}
