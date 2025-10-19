package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"sort"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

type IPalisadeProcess interface {
	Process(ctx context.Context) error
}

type PalisadeProcess struct {
	repo                  *webapi.MexcWebapi
	repoV2                *webapi.MexcV2Webapi
	traidingPairsService  *service.TradingPair
	palisadeLevelsService *service.PalisadeLevels
	buyService            *service.ByuService
	stateRepo             repo.IStateRepository
}

func NewPalisadeProcessUsecase(
	repo *webapi.MexcWebapi,
	repoV2 *webapi.MexcV2Webapi,
	traidingPairsService *service.TradingPair,
	palisadeLevelsService *service.PalisadeLevels,
	buyService *service.ByuService,
	stateRepo repo.IStateRepository,
) *PalisadeProcess {
	return &PalisadeProcess{
		repo:                  repo,
		repoV2:                repoV2,
		traidingPairsService:  traidingPairsService,
		palisadeLevelsService: palisadeLevelsService,
		buyService:            buyService,
		stateRepo:             stateRepo,
	}
}

func (u *PalisadeProcess) Process(ctx context.Context) error {
	fmt.Println("palisade process")
	accountInfo, err := u.repo.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("failed to get balance: %w", err)
	}
	fmt.Printf("accountInfo: %+v\n", accountInfo)

	pair := u.traidingPairsService.GetPalisade()
	// fmt.Printf("pair: %+v\n", pair)
	fmt.Printf("Process pair: %s\n", pair.Pair.String())

	levels, err := u.palisadeLevelsService.CheckLevels(pair, enum.D7)
	if err != nil {
		return wrap.Errorf("failed to get levels: %w", err)
	}
	// fmt.Printf("levels: %+v\n", levels)

	fmt.Printf("Found levels: %f - %f\n", levels.Min, levels.Max)

	priceCheck, err := u.palisadeLevelsService.CheckPriceInLevels(pair, levels)
	if err != nil {
		return wrap.Errorf("failed to check price in levels: %w", err)
	}

	if !priceCheck {
		fmt.Println("Price not in palisade levels, skip")
	}

	_, err = u.buyService.Buy(ctx, pair, levels)
	if err != nil {
		return wrap.Errorf("failed to byu coin: %w", err)
	}

	os.Exit(1)

	orderResult, err := u.repo.NewOrder(
		model.OrderParams{
			Symbol:           "DOLZUSDT",
			Side:             order.BUY,
			OrderType:        order.LIMIT,
			Quantity:         1799.0,
			QuoteOrderQty:    1799.0,
			Price:            0.00556,
			NewClientOrderId: "Test order 1",
		},
	)

	fmt.Printf("orderResult: %+v\n", orderResult)
	openOrders, err := u.repo.GetOpenOrders(
		ctx,
		model.OrderParams{
			Symbol: "DOLZUSDT",
		},
	)
	fmt.Printf("openOrders: %+v\n", openOrders)
	os.Exit(1)

	data, err := getCvsData()
	if err != nil {
		return wrap.Errorf("failed to get csv data: %w", err)
	}
	//fmt.Printf("data: %+v\n", data)

	sort.Slice(data, func(i, j int) bool {
		return data[i].Date.Before(data[j].Date)
	})

	//fmt.Printf("%v\n", data[0])
	//os.Exit(1)
	fmt.Printf("data: %+v\n", data)

	maxResult := 0.0
	buyIndex := 0
	sellIndex := 0

	for b := 0; b < 100; b++ {
		for s := 0; s < 100; s++ {
			result := 100.0
			inPosition := false
			buyValue := 0.0
			//lastByuPrice := 0.0
			for _, item := range data {
				if !inPosition && item.Index < b {
					inPosition = true
					buyValue = result / item.Price
					//lastByuPrice = item.Price
					fmt.Printf("Index %d price: %f amount %f\n", item.Index, item.Price, buyValue)
					//os.Exit(1)
					fmt.Println("buy")
				}
				if inPosition && item.Index > s {
					inPosition = false
					result = buyValue * item.Price
					fmt.Printf("Index %d price: %f amount %f result %f\n",
						item.Index,
						item.Price,
						buyValue,
						result,
					)
					//os.Exit(1)
					fmt.Println("sell")
				}
			}

			if result > maxResult {
				maxResult = result
				buyIndex = b
				sellIndex = s
			}
		}
	}

	fmt.Printf("result: %.2f\n", maxResult)
	fmt.Printf("buyIndex: %d\n", buyIndex)
	fmt.Printf("sellIndex: %d\n", sellIndex)

	result := 100.0
	inPosition := false
	buyValue := 0.0
	lastByuPrice := 0.0
	for _, item := range data {
		if !inPosition {
			inPosition = true
			buyValue = result / item.Price
			lastByuPrice = item.Price
			fmt.Printf("Index %d price: %f amount %f\n", item.Index, item.Price, buyValue)
			//os.Exit(1)
			fmt.Println("buy")
		}
		if inPosition && item.Price > lastByuPrice {
			inPosition = false
			result = buyValue * item.Price
			fmt.Printf("Index %d price: %f amount %f result %f\n",
				item.Index,
				item.Price,
				buyValue,
				result,
			)
			//os.Exit(1)
			fmt.Println("sell")
		}
	}

	fmt.Printf("result: %.2f\n", result)

	maxResult = 0.0
	bestPercent := 0
	result = 100.0
	inPosition = false
	buyValue = 0.0
	lastByuPrice = 0.0

	for percent := 1; percent < 100; percent++ {
		result := 100.0
		inPosition := false
		buyValue := 0.0
		lastByuPrice := 0.0
		for _, item := range data {
			if !inPosition {
				inPosition = true
				buyValue = result / item.Price
				lastByuPrice = item.Price
				fmt.Printf("Index %d price: %f amount %f\n", item.Index, item.Price, buyValue)
				//os.Exit(1)
				fmt.Println("buy")
			}
			if inPosition && item.Price > (lastByuPrice+lastByuPrice/100.0*float64(percent)) {
				inPosition = false
				result = buyValue * item.Price
				fmt.Printf("Index %d price: %f amount %f result %f\n",
					item.Index,
					item.Price,
					buyValue,
					result,
				)
				//os.Exit(1)
				fmt.Println("sell")
			}
		}
		if result > maxResult {
			maxResult = result
			bestPercent = percent
		}
	}

	fmt.Printf("result: %.2f\n", maxResult)
	fmt.Printf("percent: %d\n", bestPercent)

	maxResult = 0.0
	bestPercent = 0
	bestBuyPercent := 0
	result = 100.0
	inPosition = false
	buyValue = 0.0
	lastByuPrice = 0.0

	for buyPercent := 1; buyPercent < 30; buyPercent++ {
		for percent := 1; percent < 50; percent++ {
			result := 100.0
			inPosition := false
			buyValue := 0.0
			lastByuPrice := 3000.0
			for _, item := range data {
				if !inPosition && item.Price < (lastByuPrice-lastByuPrice/100.0*float64(buyPercent)) {
					inPosition = true
					buyValue = result / item.Price
					lastByuPrice = item.Price
					fmt.Printf("Index %d price: %f amount %f\n", item.Index, item.Price, buyValue)
					//os.Exit(1)
					fmt.Println("buy")
				}
				if inPosition && item.Price > (lastByuPrice+lastByuPrice/100.0*float64(percent)) {
					inPosition = false
					result = buyValue * item.Price
					fmt.Printf("Index %d price: %f amount %f result %f\n",
						item.Index,
						item.Price,
						buyValue,
						result,
					)
					//os.Exit(1)
					fmt.Println("sell")
				}
			}
			if result > maxResult {
				maxResult = result
				bestPercent = percent
				bestBuyPercent = buyPercent
			}
		}
	}

	fmt.Printf("result: %.2f\n", maxResult)
	fmt.Printf("percent: %d\n", bestPercent)
	fmt.Printf("bestBuyPercent: %d\n", bestBuyPercent)

	log.Println("Fear research!!!")
	return nil
}

func getCvsData() ([]model.DayInfo, error) {
	result := []model.DayInfo{}

	// open file
	//f, err := os.Open("btc_fear_greed.csv")
	//f, err := os.Open("btc_fear_greed_hour_minutes.csv")
	//f, err := os.Open("btc_fear_greed_hour_minutes_last_year.csv")
	f, err := os.Open("btc_fear_greed_hour_eth.csv")
	if err != nil {
		return nil, wrap.Errorf("failed to open csv file: %w", err)
	}

	// remember to close the file at the end of the program
	defer f.Close()

	// read csv values using csv.Reader
	csvReader := csv.NewReader(f)
	for {
		rec, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, wrap.Errorf("failed to read next string from csv file: %w", err)
		}
		// do something with read line
		fmt.Printf("%+v\n", rec)

		dayInfo, err := model.NewDayInfo(rec)
		if err != nil {
			return nil, wrap.Errorf("failed to parse dayInfo %v: %w", rec, err)
		}
		result = append(result, *dayInfo)
	}

	return result, nil
}
