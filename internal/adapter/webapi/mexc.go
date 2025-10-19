package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"mexc-sdk/mexcsdk"
	"os"

	"github.com/drybin/palisade/internal/app/cli/config"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/pkg/wrap"
	"github.com/go-resty/resty/v2"
)

const mexc_spot_account_info_url = "/api/v3/account"
const mexc_new_order_url = "/api/v3/order"

type MexcWebapi struct {
	client *resty.Client
	spot   mexcsdk.Spot
	config config.MexcConfig
}

func NewMexcWebapi(
	client *resty.Client,
	spot mexcsdk.Spot,
	config config.MexcConfig,
) *MexcWebapi {
	return &MexcWebapi{
		client: client,
		spot:   spot,
		config: config,
	}
}

func (m *MexcWebapi) GetBalance(ctx context.Context) (*mexc.SpotAccountInfo, error) {
	res := m.spot.AccountInfo()
	bytes, _ := json.Marshal(res)

	result := mexc.SpotAccountInfo{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal accountInfo info: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetAllTickerPrices(ctx context.Context) (*mexc.TickersWithPrice, error) {
	res := m.spot.TickerPrice(nil)
	bytes, _ := json.Marshal(res)

	result := mexc.TickersWithPrice{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal ticker with price info: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetSymbolInfo(ctx context.Context, symbol string) (*mexc.SymbolInfo, error) {
	options := map[string]string{
		"symbol": symbol,
	}
	res := m.spot.ExchangeInfo(options)
	bytes, _ := json.Marshal(res)

	result := mexc.SymbolInfo{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal symbol info: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) NewOrder(
	orderParams model.OrderParams,
) (*mexc.PlaceOrderResult, error) {

	options := map[string]string{
		"price":            orderParams.GetPrice(),
		"quantity":         orderParams.GetQuantity(),
		"newClientOrderId": orderParams.NewClientOrderId,
	}

	resp := m.spot.NewOrder(orderParams.GetSymbol(), orderParams.GetSide(), orderParams.GetOrderType(), options)

	if resp == nil {
		return nil, wrap.Errorf("failed to place order on mexc: %w", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.PlaceOrderResult{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal order info: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetOpenOrders(
	ctx context.Context,
	orderParams model.OrderParams,
) (*mexc.OpenOrders, error) {

	resp := m.spot.OpenOrders(orderParams.GetSymbol())

	if resp == nil {
		return nil, wrap.Errorf("failed to get open orders: %w", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.OpenOrders{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal open orders: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetKlines(
	pair model.PairWithLevels,
	interval enum.KlineInterval,
) (*mexc.OpenOrders, error) {
	symbol := pair.Pair.String()

	options := map[string]string{
		"limit": "700",
	}

	resp := m.spot.Klines(&symbol, interval.StringPtr(), options)
	fmt.Printf("resp: %+v\n", resp)
	os.Exit(1)

	if resp == nil {
		return nil, wrap.Errorf("failed to get open orders: %w", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.OpenOrders{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal open orders: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetTicker24hr(
	pair model.PairWithLevels,
	interval enum.KlineInterval,
) (*mexc.OpenOrders, error) {
	//symbol := pair.Pair.String()
	//
	//options := map[string]string{
	//    "limit": "700",
	//}

	resp := m.spot.Ticker24hr(pair.Pair.CoinFirst.StringPtr())
	//resp := m.spot.Klines(&symbol, interval.StringPtr(), options)
	fmt.Printf("resp: %+v\n", resp)
	os.Exit(1)

	if resp == nil {
		return nil, wrap.Errorf("failed to get open orders: %w", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.OpenOrders{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal open orders: %w", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetTrades(
	pair model.PairWithLevels,
	interval enum.KlineInterval,
) (*mexc.OpenOrders, error) {
	//symbol := pair.Pair.String()
	//
	options := map[string]string{
		"limit": "700",
	}

	//resp := m.spot.Ticker24hr(pair.Pair.CoinFirst.StringPtr())
	resp := m.spot.Trades(pair.Pair.CoinFirst.StringPtr(), &options)
	//resp := m.spot.Klines(&symbol, interval.StringPtr(), options)
	fmt.Printf("resp: %+v\n", resp)
	os.Exit(1)

	if resp == nil {
		return nil, wrap.Errorf("failed to get open orders: %w", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.OpenOrders{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal open orders: %w", err)
	}

	return &result, nil
}
