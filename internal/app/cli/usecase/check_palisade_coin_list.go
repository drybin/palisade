package usecase

import (
	"context"
	"fmt"
	"time"

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
	fmt.Println("get coin list")

	res, err := u.repo.GetAllTickerPrices(ctx)
	if err != nil {
		return wrap.Errorf("failed get all tickers prices: %w", err)
	}
	//fmt.Printf("%+v\n", res)

	fmt.Printf("Found pairs count: %d\n", len(*res))

	for _, symbol := range *res {
		fmt.Println("Check symbol: " + symbol.Symbol)

		symbolFromDb, err := u.stateRepo.GetCoinInfo(ctx, symbol.Symbol)
		if err != nil {
			return wrap.Errorf("failed get symbol details from db: %w", err)
		}
		if symbolFromDb != nil {
			fmt.Println("Symbol found in db, continue")
			continue
		}

		r, err := u.repo.GetSymbolInfo(ctx, symbol.Symbol)
		if err != nil {
			return wrap.Errorf("failed get ticker details: %w", err)
		}
		//fmt.Printf("%+v\n", r)
		err = u.stateRepo.SaveCoin(ctx, &r.Symbols[0])
		if err != nil {
			return wrap.Errorf("failed to save coin: %w", err)
		}

		fmt.Println("ok")
		time.Sleep(2 * time.Second)
	}
	return nil
}
