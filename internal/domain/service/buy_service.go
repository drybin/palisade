package service

import (
	"context"
	"fmt"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const amountInUSDT = 10.0

type ByuService struct {
	api       *webapi.MexcWebapi
	apiV2     *webapi.MexcV2Webapi
	stateRepo repo.IStateRepository
}

func NewByuService(api *webapi.MexcWebapi, apiV2 *webapi.MexcV2Webapi, stateRepo repo.IStateRepository) *ByuService {
	return &ByuService{api: api, apiV2: apiV2, stateRepo: stateRepo}
}

func (s *ByuService) Buy(
	ctx context.Context,
	pair model.PairWithLevels,
	levels *model.ArrayStats,
) (*model.ArrayStats, error) {
	amount := calcAmount(levels.Min)
	fmt.Printf("Amount to buy: %f\n", amount)

	clientOrderId, err := s.getDevOrderClientId(ctx, pair)
	if err != nil {
		return nil, wrap.Errorf("failed to generate client order id: %w", err)
	}
	fmt.Println("Generated client order id: " + *clientOrderId)

	orderResult, err := s.api.NewOrder(
		model.OrderParams{
			Symbol:           "DOLZUSDT",
			Side:             order.BUY,
			OrderType:        order.LIMIT,
			Quantity:         amount,
			QuoteOrderQty:    amount,
			Price:            levels.Min,
			NewClientOrderId: *clientOrderId,
		},
	)
	if err != nil {
		return nil, wrap.Errorf("failed to place order: %w", err)
	}

	fmt.Printf("OrderResult: %+v\n", orderResult)

	//if err != nil {
	//	return nil, wrap.Errorf("failed to check price in levels: %w", err)
	//}
	//
	//fmt.Printf("orderResult: %+v\n", orderResult)
	//openOrders, err := u.repo.GetOpenOrders(
	//	ctx,
	//	model.OrderParams{
	//		Symbol: "DOLZUSDT",
	//	},
	//)
	//fmt.Printf("openOrders: %+v\n", openOrders)

	return nil, nil
}

func calcAmount(price float64) float64 {
	return amountInUSDT / price
}

func (s *ByuService) getDevOrderClientId(ctx context.Context, pair model.PairWithLevels) (*string, error) {
	logCount, err := s.stateRepo.GetCountLogsByCoin(ctx, pair.Pair.CoinFirst, pair.Pair.CoinSecond)
	if err != nil {
		return nil, wrap.Errorf("failed to get logs count: %w", err)
	}

	res := fmt.Sprintf("bot_dev_%s_%s_number_%d_debug", pair.Pair.CoinFirst, pair.Pair.CoinSecond, *logCount)
	return &res, nil
}
