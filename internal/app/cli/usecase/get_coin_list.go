package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type IGetCoinList interface {
	Process(ctx context.Context, debug bool) error
}

type GetCoinList struct {
	repo      *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewGetCoinListUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
) *GetCoinList {
	return &GetCoinList{
		repo:      repo,
		stateRepo: stateRepo,
	}
}

func (u *GetCoinList) Process(ctx context.Context, debug bool) error {
	startTime := time.Now()
	fmt.Printf("Get all coin list and save to db\n")
	fmt.Printf("Время начала обработки: %s\n", startTime.Format("2006-01-02 15:04:05"))

	res, err := u.repo.GetAllTickerPrices(ctx)
	if err != nil {
		return wrap.Errorf("failed get all tickers prices: %w", err)
	}
	//fmt.Printf("%+v\n", res)

	fmt.Printf("Found pairs count: %d\n", len(*res))

	newCoinsCount := 0
	totalCount := len(*res)
	currentIndex := 0
	for _, symbol := range *res {
		currentIndex++
		if debug {
			fmt.Printf("[%d/%d] Обрабатываем монету: %s\n", currentIndex, totalCount, symbol.Symbol)
		}
		// fmt.Println("Check symbol: " + symbol.Symbol)

		symbolFromDb, err := u.stateRepo.GetCoinInfo(ctx, symbol.Symbol)
		if err != nil {
			return wrap.Errorf("failed get symbol details from db: %w", err)
		}
		if symbolFromDb != nil {
			// fmt.Println("Symbol found in db, continue")
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

		newCoinsCount++
		// fmt.Println("ok")
		time.Sleep(2 * time.Second)
	}

	elapsed := time.Since(startTime)
	fmt.Println("All done")
	fmt.Printf("Время начала обработки: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Время окончания обработки: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Общее время обработки: %v\n", elapsed)
	fmt.Printf("Количество новых найденных монет: %d\n", newCoinsCount)
	return nil
}
