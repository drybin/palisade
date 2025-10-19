package service

import (
	"fmt"
	"strconv"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/helpers"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/pkg/wrap"
)

type PalisadeLevels struct {
	api   *webapi.MexcWebapi
	apiV2 *webapi.MexcV2Webapi
}

func NewPalisadeLevels(api *webapi.MexcWebapi, apiV2 *webapi.MexcV2Webapi) *PalisadeLevels {
	return &PalisadeLevels{api: api, apiV2: apiV2}
}

func (s *PalisadeLevels) CheckLevels(
	pair model.PairWithLevels,
	interval enum.KlineIntervalV2,
) (*model.ArrayStats, error) {
	// fmt.Println("Check Levels")

	res, err := s.apiV2.GetKLines(pair, interval.String(), pair.Pair.CoinFirst.SymbolId)
	if err != nil {
		return nil, wrap.Errorf("failed to get klines: %w", err)
	}
	//fmt.Printf("%+v\n", res)

	floatPrices, err := helpers.StringsToFloats(res.Data.O)
	if err != nil {
		return nil, wrap.Errorf("failed to parse prices: %w", err)
	}
	levels := helpers.AnalyzeFloatArrayWithThreshold(floatPrices, 0.3)

	// fmt.Printf("levels %v\n", levels)

	return &levels, nil
}

func (s *PalisadeLevels) CheckPriceInLevels(pair model.PairWithLevels, levels *model.ArrayStats) (bool, error) {
	currentPrice, err := s.apiV2.GetPrice(pair.Pair.CoinFirst.CoinId, pair.Pair.CoinFirst.SymbolId)
	if err != nil {
		return false, wrap.Errorf("failed to get price: %w", err)
	}

	priceFloat, err := strconv.ParseFloat(currentPrice.Data.MarketInformation.CurrentPrice, 64)

	if err != nil {
		return false, wrap.Errorf("Error converting current price to float64: %v\n", err)
	}
	// priceFloat := currentPrice.Data.MarketInformation.CurrentPrice
	// fmt.Printf("CurrentPrice %+v\n", currentPrice)
	fmt.Printf("CurrentPrice %f\n", priceFloat)

	if !(priceFloat >= levels.Min && priceFloat <= levels.Max) {
		fmt.Println("Price not in palisade levels, skip")
		return false, nil
	}

	return true, nil
}
