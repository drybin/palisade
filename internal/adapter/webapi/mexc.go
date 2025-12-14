package webapi

import (
	"context"
	"encoding/json"
	"mexc-sdk/mexcsdk"
	"strconv"

	"github.com/drybin/palisade/internal/app/cli/config"
	"github.com/drybin/palisade/internal/domain/enum"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/pkg/wrap"
	"github.com/go-resty/resty/v2"
)

// const mexc_spot_account_info_url = "/api/v3/account"
// const mexc_new_order_url = "/api/v3/order"

type MexcWebapi struct {
	client       *resty.Client
	publicClient *resty.Client
	spot         mexcsdk.Spot
	config       config.MexcConfig
}

func NewMexcWebapi(
	client *resty.Client,
	spot mexcsdk.Spot,
	config config.MexcConfig,
) *MexcWebapi {
	// Создаем отдельный клиент для публичных запросов без заголовка X-MEXC-APIKEY
	publicClient := resty.New()
	publicClient.SetBaseURL(config.BaseUrl)
	publicClient.SetHeader("Content-Type", "application/json")
	// Не устанавливаем X-MEXC-APIKEY для публичных запросов

	return &MexcWebapi{
		client:       client,
		publicClient: publicClient,
		spot:         spot,
		config:       config,
	}
}

func (m *MexcWebapi) GetBalance(ctx context.Context) (*mexc.AccountInfo, error) {
	res := m.spot.AccountInfo()
	bytes, _ := json.Marshal(res)

	spotAccountInfo := mexc.SpotAccountInfo{}
	err := json.Unmarshal(bytes, &spotAccountInfo)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal accountInfo info: %w", err)
	}

	accountInfo := mapSpotAccountInfoToAccountInfo(spotAccountInfo)
	return accountInfo, nil
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
		return nil, wrap.Errorf("failed to place order on mexc: %v", resp)
	}

	bytes, _ := json.Marshal(resp)

	result := mexc.PlaceOrderResult{}
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal order info: %v", err)
	}

	return &result, nil
}

func (m *MexcWebapi) CancelOrder(
	symbol string,
	orderId string,
) (*mexc.CancelOrderResponse, error) {
	// Параметры отмены
	params := map[string]string{
		"symbol":  symbol,
		"orderId": orderId, // ID ордера
	}

	// Вызов cancel
	resp := m.spot.CancelOrder(&symbol, params)

	if resp == nil {
		return nil, wrap.Errorf("failed to cancel order: response is nil")
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		return nil, wrap.Errorf("failed to marshal cancel order response: %w", err)
	}

	result, err := mexc.ParseCancelOrderResponseFromJSON(bytes)
	if err != nil {
		return nil, wrap.Errorf("failed to parse cancel order response: %w", err)
	}

	return result, nil
}

func (m *MexcWebapi) GetOpenOrders(
	ctx context.Context,
	orderParams model.OrderParams,
) (*mexc.OpenOrders, error) {

	resp := m.spot.OpenOrders(orderParams.GetSymbol())

	if resp == nil {
		return nil, wrap.Errorf("failed to get open orders: %v", resp)
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		return nil, wrap.Errorf("failed to marshal open orders response: %v", err)
	}

	result, err := mexc.ParseOpenOrdersFromJSON(bytes)
	if err != nil {
		return nil, wrap.Errorf("failed to parse open orders: %v", err)
	}

	return &result, nil
}

func (m *MexcWebapi) GetOrderQuery(
	symbol string,
	orderId string,
) (*mexc.QueryOrderResult, error) {
	params := map[string]string{
		"symbol":  symbol,
		"orderId": orderId, // ID ордера
	}

	resp := m.spot.QueryOrder(&symbol, params)

	if resp == nil {
		return nil, wrap.Errorf("failed to query order: response is nil")
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		return nil, wrap.Errorf("failed to marshal query order response: %w", err)
	}

	result, err := mexc.ParseQueryOrderFromJSON(bytes)
	if err != nil {
		return nil, wrap.Errorf("failed to parse query order response: %w", err)
	}

	return result, nil
}

func (m *MexcWebapi) GetKlines(
	pair model.PairWithLevels,
	interval enum.KlineInterval,
) (*mexc.Klines, error) {
	symbol := pair.Pair.String()
	intervalStr := interval.String()

	// Используем прямой HTTP запрос для публичного endpoint /api/v3/klines
	// Используем publicClient без заголовка X-MEXC-APIKEY
	res, err := m.publicClient.R().
		SetQueryParams(map[string]string{
			"symbol":   symbol,
			"interval": intervalStr,
			"limit":    "700",
		}).
		Get("/api/v3/klines")

	if err != nil {
		return nil, wrap.Errorf("failed to get klines: %w", err)
	}

	if res.IsError() {
		return nil, wrap.Errorf("failed to get klines, status: %d, body: %s", res.StatusCode(), string(res.Body()))
	}

	klines, err := mexc.ParseKlinesFromJSON(res.Body())
	if err != nil {
		return nil, wrap.Errorf("failed to parse klines: %w", err)
	}

	return &klines, nil
}

func (m *MexcWebapi) GetAvgPrice(ctx context.Context, symbol string) (*mexc.AvgPrice, error) {
	// Используем прямой HTTP запрос для публичного endpoint /api/v3/avgPrice
	// Используем publicClient без заголовка X-MEXC-APIKEY
	res, err := m.publicClient.R().
		SetQueryParams(map[string]string{
			"symbol": symbol,
		}).
		Get("/api/v3/avgPrice")

	if err != nil {
		return nil, wrap.Errorf("failed to get avg price: %w", err)
	}

	if res.IsError() {
		return nil, wrap.Errorf("failed to get avg price, status: %d, body: %s", res.StatusCode(), string(res.Body()))
	}

	var response struct {
		Mins  int    `json:"mins"`
		Price string `json:"price"`
	}
	err = json.Unmarshal(res.Body(), &response)
	if err != nil {
		return nil, wrap.Errorf("failed to unmarshal avg price info: %w", err)
	}

	price, err := strconv.ParseFloat(response.Price, 64)
	if err != nil {
		return nil, wrap.Errorf("failed to parse price: %w", err)
	}

	result := mexc.AvgPrice{
		Mins:  response.Mins,
		Price: price,
	}

	return &result, nil
}

func mapSpotAccountInfoToAccountInfo(spotInfo mexc.SpotAccountInfo) *mexc.AccountInfo {
	accountInfo := &mexc.AccountInfo{
		CanTrade:    spotInfo.CanTrade,
		CanWithdraw: spotInfo.CanWithdraw,
		CanDeposit:  spotInfo.CanDeposit,
		AccountType: spotInfo.AccountType,
		Permissions: spotInfo.Permissions,
	}

	// Конвертируем комиссии из interface{} в int64
	if makerComm, ok := spotInfo.MakerCommission.(float64); ok {
		accountInfo.MakerCommission = int64(makerComm)
	} else if makerComm, ok := spotInfo.MakerCommission.(int64); ok {
		accountInfo.MakerCommission = makerComm
	} else if makerCommStr, ok := spotInfo.MakerCommission.(string); ok {
		if val, err := strconv.ParseInt(makerCommStr, 10, 64); err == nil {
			accountInfo.MakerCommission = val
		}
	}

	if takerComm, ok := spotInfo.TakerCommission.(float64); ok {
		accountInfo.TakerCommission = int64(takerComm)
	} else if takerComm, ok := spotInfo.TakerCommission.(int64); ok {
		accountInfo.TakerCommission = takerComm
	} else if takerCommStr, ok := spotInfo.TakerCommission.(string); ok {
		if val, err := strconv.ParseInt(takerCommStr, 10, 64); err == nil {
			accountInfo.TakerCommission = val
		}
	}

	if buyerComm, ok := spotInfo.BuyerCommission.(float64); ok {
		accountInfo.BuyerCommission = int64(buyerComm)
	} else if buyerComm, ok := spotInfo.BuyerCommission.(int64); ok {
		accountInfo.BuyerCommission = buyerComm
	} else if buyerCommStr, ok := spotInfo.BuyerCommission.(string); ok {
		if val, err := strconv.ParseInt(buyerCommStr, 10, 64); err == nil {
			accountInfo.BuyerCommission = val
		}
	}

	if sellerComm, ok := spotInfo.SellerCommission.(float64); ok {
		accountInfo.SellerCommission = int64(sellerComm)
	} else if sellerComm, ok := spotInfo.SellerCommission.(int64); ok {
		accountInfo.SellerCommission = sellerComm
	} else if sellerCommStr, ok := spotInfo.SellerCommission.(string); ok {
		if val, err := strconv.ParseInt(sellerCommStr, 10, 64); err == nil {
			accountInfo.SellerCommission = val
		}
	}

	// Конвертируем UpdateTime из interface{} в int64
	if updateTime, ok := spotInfo.UpdateTime.(float64); ok {
		accountInfo.UpdateTime = int64(updateTime)
	} else if updateTime, ok := spotInfo.UpdateTime.(int64); ok {
		accountInfo.UpdateTime = updateTime
	} else if updateTimeStr, ok := spotInfo.UpdateTime.(string); ok {
		if val, err := strconv.ParseInt(updateTimeStr, 10, 64); err == nil {
			accountInfo.UpdateTime = val
		}
	}

	// Конвертируем балансы
	accountInfo.Balances = make([]mexc.Balance, 0, len(spotInfo.Balances))
	for _, balance := range spotInfo.Balances {
		free, _ := strconv.ParseFloat(balance.Free, 64)
		locked, _ := strconv.ParseFloat(balance.Locked, 64)
		available, _ := strconv.ParseFloat(balance.Available, 64)

		accountInfo.Balances = append(accountInfo.Balances, mexc.Balance{
			Asset:     balance.Asset,
			Free:      free,
			Locked:    locked,
			Available: available,
		})
	}

	return accountInfo
}
