package usecase

import (
	"context"
	"fmt"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckPalisadeCoinList interface {
	Process(ctx context.Context) error
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

func (u *CheckPalisadeCoinList) Process(ctx context.Context) error {
	fmt.Println("check palisade")
	spotTradingAllowed := true
	isPalisade := false
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
	for _, coin := range data {

	}

	return nil
}
