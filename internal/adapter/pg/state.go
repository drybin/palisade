package registry

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
	palisade_database "github.com/drybin/palisade/sqlc/gen"
	"github.com/jackc/pgx/v5"
)

type StateRepository struct {
	Postgree *pgx.Conn
}

func NewStateRepository(pg *pgx.Conn) StateRepository {
	return StateRepository{
		Postgree: pg,
	}
}

func (u StateRepository) GetCoinState(
	ctx context.Context,
	coinFirst model.Coin,
	coinSecond model.Coin,
) (*model.State, error) {
	db := palisade_database.New(u.Postgree)
	state, err := db.GetCoinState(
		ctx,
		palisade_database.GetCoinStateParams{
			Coinfirst:  coinFirst.String(),
			Coinsecond: coinSecond.String(),
		},
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("failed to get state from Postgree: %w", err)
	}
	return mapToDomainModel(state), nil
}

func (u StateRepository) GetCountLogsByCoin(
	ctx context.Context,
	coinFirst model.Coin,
	coinSecond model.Coin,
) (*int, error) {
	db := palisade_database.New(u.Postgree)
	count, err := db.GetCountLogsByCoin(
		ctx,
		palisade_database.GetCountLogsByCoinParams{
			Coinfirst:  coinFirst.String(),
			Coinsecond: coinSecond.String(),
		},
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("failed to logs count from Postgree: %w", err)
	}
	res := int(count)
	return &res, nil
}

func (u StateRepository) SaveCoin(
	ctx context.Context,
	model *mexc.SymbolDetail,
) error {
	db := palisade_database.New(u.Postgree)

	data, err := mapSymbolDetailToSaveCoinParam(model)
	if err != nil {
		return wrap.Errorf("failed to map SymbolDetail to db param: %w", err)
	}

	_, err = db.SaveCoin(
		ctx,
		*data,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return wrap.Errorf("failed to get state from Postgree: %w", err)
	}
	return nil
}

func (u StateRepository) GetCoinInfo(
	ctx context.Context,
	symbol string,
) (*mexc.SymbolDetail, error) {
	db := palisade_database.New(u.Postgree)

	coinInfo, err := db.GetCoinInfo(ctx, symbol)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("failed to get coin info from Postgree: %w", err)
	}

	return mapCoinToDomainModel(coinInfo)
}

func (u StateRepository) GetCoins(
	ctx context.Context,
	params repo.GetCoinsParams,
) ([]mexc.SymbolDetail, error) {
	db := palisade_database.New(u.Postgree)

	// Преобразуем *bool в bool (если nil, используем false)
	isSpotTradingAllowed := false
	if params.IsSpotTradingAllowed != nil {
		isSpotTradingAllowed = *params.IsSpotTradingAllowed
	}

	isPalisade := false
	if params.IsPalisade != nil {
		isPalisade = *params.IsPalisade
	}

	coins, err := db.GetCoins(ctx, palisade_database.GetCoinsParams{
		Limit:   int32(params.Limit),
		Offset:  int32(params.Offset),
		Column3: isSpotTradingAllowed,
		Column4: isPalisade,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []mexc.SymbolDetail{}, nil
		}
		return nil, wrap.Errorf("failed to get coins from Postgree: %w", err)
	}

	result := make([]mexc.SymbolDetail, 0, len(coins))
	for _, coin := range coins {
		symbolDetail, err := mapCoinToDomainModel(coin)
		if err != nil {
			return nil, wrap.Errorf("failed to map coin to domain model: %w", err)
		}
		result = append(result, *symbolDetail)
	}

	return result, nil
}

func (u StateRepository) UpdateIsPalisade(
	ctx context.Context,
	symbol string,
	isPalisade bool,
) error {
	db := palisade_database.New(u.Postgree)

	timeNow := time.Now()
	err := db.UpdateIsPalisade(ctx, palisade_database.UpdateIsPalisadeParams{
		Ispalisade: isPalisade,
		Lastcheck:  &timeNow,
		Symbol:     symbol,
	})

	if err != nil {
		return wrap.Errorf("failed to update isPalisade for coin %s: %w", symbol, err)
	}

	return nil
}

func (u StateRepository) UpdatePalisadeParams(
	ctx context.Context,
	symbol string,
	support, resistance, rangeValue, rangePercent, avgPrice, volatility, maxDrawdown, maxRise float64,
) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdatePalisadeParams(ctx, palisade_database.UpdatePalisadeParamsParams{
		Support:      &support,
		Resistance:   &resistance,
		Rangevalue:   &rangeValue,
		Rangepercent: &rangePercent,
		Avgprice:     &avgPrice,
		Volatility:   &volatility,
		Maxdrawdown:  &maxDrawdown,
		Maxrise:      &maxRise,
		Symbol:       symbol,
	})

	if err != nil {
		return wrap.Errorf("failed to update palisade params for coin %s: %w", symbol, err)
	}

	return nil
}

func (u StateRepository) GetCoinsToProcess(
	ctx context.Context,
	limit int,
	offset int,
) ([]mexc.SymbolDetail, error) {
	db := palisade_database.New(u.Postgree)

	coins, err := db.GetCoinsToProcess(ctx, palisade_database.GetCoinsToProcessParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []mexc.SymbolDetail{}, nil
		}
		return nil, wrap.Errorf("failed to get coins to process from Postgree: %w", err)
	}

	result := make([]mexc.SymbolDetail, 0, len(coins))
	for _, coin := range coins {
		symbolDetail, err := mapCoinToDomainModel(coin)
		if err != nil {
			return nil, wrap.Errorf("failed to map coin to domain model: %w", err)
		}
		result = append(result, *symbolDetail)
	}

	return result, nil
}

func (u StateRepository) GetCoinsToProcessTPTU(
	ctx context.Context,
	limit int,
	offset int,
) ([]mexc.SymbolDetail, error) {
	db := palisade_database.New(u.Postgree)

	coins, err := db.GetCoinsToProcessTPTU(ctx, palisade_database.GetCoinsToProcessTPTUParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []mexc.SymbolDetail{}, nil
		}
		return nil, wrap.Errorf("failed to get coins to process TPTU from Postgree: %w", err)
	}

	result := make([]mexc.SymbolDetail, 0, len(coins))
	for _, coin := range coins {
		symbolDetail, err := mapCoinToDomainModel(coin)
		if err != nil {
			return nil, wrap.Errorf("failed to map coin to domain model: %w", err)
		}
		result = append(result, *symbolDetail)
	}

	return result, nil
}

func (u StateRepository) GetNextTradeId(ctx context.Context) (int, error) {
	db := palisade_database.New(u.Postgree)

	lastTradeId, err := db.GetLastTradeId(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Если нет записей, возвращаем 1
			return 1, nil
		}
		return 0, wrap.Errorf("failed to get last trade id from Postgree: %w", err)
	}

	// Если результат NULL, возвращаем 1
	if lastTradeId == nil {
		return 1, nil
	}

	// Конвертируем interface{} в int
	var maxId int
	switch v := lastTradeId.(type) {
	case int64:
		maxId = int(v)
	case int32:
		maxId = int(v)
	case int:
		maxId = v
	case float64:
		maxId = int(v)
	default:
		// Пытаемся конвертировать через строку
		maxIdStr := fmt.Sprintf("%v", lastTradeId)
		parsed, err := strconv.Atoi(maxIdStr)
		if err != nil {
			return 0, wrap.Errorf("failed to convert last trade id to int: %w", err)
		}
		maxId = parsed
	}

	// Возвращаем +1 от максимального ID
	return maxId + 1, nil
}

func (u StateRepository) SaveTradeLog(ctx context.Context, params repo.SaveTradeLogParams) (*repo.TradeLog, error) {
	db := palisade_database.New(u.Postgree)

	tradeLog, err := db.SaveTradeLog(ctx, palisade_database.SaveTradeLogParams{
		OpenDate:    params.OpenDate,
		OpenBalance: params.OpenBalance,
		Symbol:      params.Symbol,
		BuyPrice:    params.BuyPrice,
		Amount:      params.Amount,
		Orderid:     params.OrderId,
		Uplevel:     params.UpLevel,
		Downlevel:   params.DownLevel,
	})
	if err != nil {
		return nil, wrap.Errorf("failed to save trade log: %w", err)
	}

	return mapTradeLogToDomainModel(tradeLog), nil
}

func (u StateRepository) UpdateDealDateTradeLog(ctx context.Context, id int, dealDate time.Time) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateDealDateTradeLog(ctx, palisade_database.UpdateDealDateTradeLogParams{
		ID:       id,
		DealDate: &dealDate,
	})
	if err != nil {
		return wrap.Errorf("failed to update deal date for trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) UpdateCancelDateTradeLog(ctx context.Context, id int, cancelDate time.Time) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateCancelDateTradeLog(ctx, palisade_database.UpdateCancelDateTradeLogParams{
		ID:         id,
		CancelDate: &cancelDate,
	})
	if err != nil {
		return wrap.Errorf("failed to update cancel date for trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) UpdateSuccesTradeLog(ctx context.Context, id int, closeDate time.Time, closeBalance float64, sellPrice float64) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateSuccesTradeLog(ctx, palisade_database.UpdateSuccesTradeLogParams{
		ID:           id,
		CloseDate:    &closeDate,
		CloseBalance: &closeBalance,
		SellPrice:    &sellPrice,
	})
	if err != nil {
		return wrap.Errorf("failed to update success trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) UpdateSellOrderIdTradeLog(ctx context.Context, id int, sellOrderId string) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateSellOrderIdTradeLog(ctx, palisade_database.UpdateSellOrderIdTradeLogParams{
		ID:          id,
		OrderidSell: &sellOrderId,
	})
	if err != nil {
		return wrap.Errorf("failed to update sell order id for trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) GetOpenOrders(ctx context.Context) ([]repo.TradeLog, error) {
	db := palisade_database.New(u.Postgree)

	tradeLogs, err := db.GetOpenOrders(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []repo.TradeLog{}, nil
		}
		return nil, wrap.Errorf("failed to get open orders from Postgree: %w", err)
	}

	result := make([]repo.TradeLog, 0, len(tradeLogs))
	for _, tradeLog := range tradeLogs {
		domainTradeLog := mapTradeLogToDomainModel(tradeLog)
		result = append(result, *domainTradeLog)
	}

	return result, nil
}

func mapTradeLogToDomainModel(t palisade_database.TradeLog) *repo.TradeLog {
	closeBalance := 0.0
	if t.CloseBalance != nil {
		closeBalance = *t.CloseBalance
	}
	sellPrice := 0.0
	if t.SellPrice != nil {
		sellPrice = *t.SellPrice
	}

	orderIdSell := ""
	if t.OrderidSell != nil {
		orderIdSell = *t.OrderidSell
	}

	return &repo.TradeLog{
		ID:           t.ID,
		OpenDate:     t.OpenDate,
		DealDate:     t.DealDate,
		CloseDate:    t.CloseDate,
		CancelDate:   t.CancelDate,
		OpenBalance:  t.OpenBalance,
		CloseBalance: closeBalance,
		Symbol:       t.Symbol,
		BuyPrice:     t.BuyPrice,
		SellPrice:    sellPrice,
		Amount:       t.Amount,
		OrderId:      t.Orderid,
		OrderId_sell: orderIdSell,
		UpLevel:      t.Uplevel,
		DownLevel:    t.Downlevel,
	}
}

func mapCoinToDomainModel(c palisade_database.Coin) (*mexc.SymbolDetail, error) {
	return &mexc.SymbolDetail{
		Symbol:                   c.Symbol,
		Status:                   strconv.Itoa(c.Status),
		BaseAsset:                c.Baseasset,
		BaseAssetPrecision:       c.Baseassetprecision,
		QuoteAsset:               c.Quoteasset,
		QuotePrecision:           c.Quoteprecision,
		QuoteAssetPrecision:      c.Quoteassetprecision,
		BaseCommissionPrecision:  c.Basecommissionprecision,
		QuoteCommissionPrecision: c.Quotecommissionprecision,
		OrderTypes:               c.Ordertypes,
		IsSpotTradingAllowed:     c.Isspottradingallowed,
		IsMarginTradingAllowed:   c.Ismargintradingallowed,
		QuoteAmountPrecision:     fmt.Sprintf("%f", c.Quoteamountprecision),
		BaseSizePrecision:        strconv.FormatFloat(c.Basesizeprecision, 'f', -1, 64),
		Permissions:              c.Permissions,
		Filters: []struct {
			FilterType        string `json:"filterType"`
			BidMultiplierUp   string `json:"bidMultiplierUp"`
			AskMultiplierDown string `json:"askMultiplierDown"`
		}{},
		MaxQuoteAmount:             strconv.Itoa(c.Maxquoteamount),
		MakerCommission:            strconv.FormatFloat(c.Makercommission, 'f', -1, 64),
		TakerCommission:            strconv.FormatFloat(c.Takercommission, 'f', -1, 64),
		QuoteAmountPrecisionMarket: fmt.Sprintf("%f", c.Quoteamountprecisionmarket),
		MaxQuoteAmountMarket:       strconv.Itoa(c.Maxquoteamountmarket),
		FullName:                   c.Fullname,
		TradeSideType:              c.Tradesidetype,
		ContractAddress:            "",
		St:                         false,
		LastCheck:                  getLastCheck(c),
		IsPalisade:                 c.Ispalisade,
		Support:                    getFloat64FromPointer(c.Support),
		Resistance:                 getFloat64FromPointer(c.Resistance),
		RangeValue:                 getFloat64FromPointer(c.Rangevalue),
		RangePercent:               getFloat64FromPointer(c.Rangepercent),
		AvgPrice:                   getFloat64FromPointer(c.Avgprice),
		Volatility:                 getFloat64FromPointer(c.Volatility),
		MaxDrawdown:                getFloat64FromPointer(c.Maxdrawdown),
		MaxRise:                    getFloat64FromPointer(c.Maxrise),
	}, nil
}

// getLastCheck возвращает Lastcheck, если он не nil, иначе возвращает Date
func getLastCheck(c palisade_database.Coin) time.Time {
	if c.Lastcheck != nil {
		return *c.Lastcheck
	}
	return c.Date
}

// getFloat64FromPointer конвертирует *float64 в float64 (возвращает 0.0 если nil)
func getFloat64FromPointer(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0.0
}

func mapToDomainModel(m palisade_database.State) *model.State {
	return &model.State{
		ID:             m.ID,
		Date:           m.Date,
		AccountBalance: m.AccountBalance,
		CoinFirst:      m.Coinfirst,
		CoinSecond:     m.Coinsecond,
		Price:          m.Price,
		Amount:         m.Amount,
		State:          m.State,
		Orderid:        m.Orderid,
		Uplevel:        m.Uplevel,
		Downlevel:      m.Downlevel,
	}
}

func mapSymbolDetailToSaveCoinParam(s *mexc.SymbolDetail) (*palisade_database.SaveCoinParams, error) {
	status, err := strconv.Atoi(s.Status)

	if err != nil {
		return nil, wrap.Errorf("Error converting status to integer: %v", err)
	}

	//quoteAmountPrecision, err := strconv.Atoi(s.QuoteAmountPrecision)
	//if err != nil {
	//	return nil, wrap.Errorf("Error converting QuoteAmountPrecision to integer: %v", err)
	//}

	quoteAmountPrecision, err := strconv.ParseFloat(s.QuoteAmountPrecision, 64)
	if err != nil {
		return nil, wrap.Errorf("Error converting MakerCommission to float64: %w", err)
	}

	quoteAmountPrecisionMarket, err := strconv.ParseFloat(s.QuoteAmountPrecisionMarket, 64)
	if err != nil {
		return nil, wrap.Errorf("Error converting MakerCommission to float64: %w", err)
	}

	baseSizePrecision, err := strconv.ParseFloat(s.BaseSizePrecision, 64)
	if err != nil {
		return nil, wrap.Errorf("Error converting BaseSizePrecision to float64: %w", err)
	}

	maxQuoteAmount, err := strconv.Atoi(s.MaxQuoteAmount)
	if err != nil {
		return nil, wrap.Errorf("Error converting MaxQuoteAmount to integer: %v", err)
	}

	makerCommission, err := strconv.ParseFloat(s.MakerCommission, 64)
	if err != nil {
		return nil, wrap.Errorf("Error converting MakerCommission to float64: %w", err)
	}

	takerCommission, err := strconv.ParseFloat(s.TakerCommission, 64)
	if err != nil {
		return nil, wrap.Errorf("Error converting TakerCommission to float64: %w", err)
	}
	//
	//quoteAmountPrecisionMarket, err := strconv.Atoi(s.QuoteAmountPrecisionMarket)
	//if err != nil {
	//	return nil, wrap.Errorf("Error converting QuoteAmountPrecisionMarket to integer: %v", err)
	//}

	maxQuoteAmountMarket, err := strconv.Atoi(s.MaxQuoteAmountMarket)
	if err != nil {
		return nil, wrap.Errorf("Error converting MaxQuoteAmountMarket to integer: %v", err)
	}

	return &palisade_database.SaveCoinParams{
		Date:                       time.Now(),
		Symbol:                     s.Symbol,
		Status:                     status,
		Baseasset:                  s.BaseAsset,
		Baseassetprecision:         s.BaseAssetPrecision,
		Quoteasset:                 s.QuoteAsset,
		Quoteprecision:             s.QuotePrecision,
		Quoteassetprecision:        s.QuoteAssetPrecision,
		Basecommissionprecision:    s.BaseCommissionPrecision,
		Quotecommissionprecision:   s.QuoteCommissionPrecision,
		Ordertypes:                 s.OrderTypes,
		Isspottradingallowed:       s.IsSpotTradingAllowed,
		Ismargintradingallowed:     s.IsMarginTradingAllowed,
		Quoteamountprecision:       quoteAmountPrecision,
		Basesizeprecision:          baseSizePrecision,
		Permissions:                s.Permissions,
		Maxquoteamount:             maxQuoteAmount,
		Makercommission:            makerCommission,
		Takercommission:            takerCommission,
		Quoteamountprecisionmarket: quoteAmountPrecisionMarket,
		Maxquoteamountmarket:       maxQuoteAmountMarket,
		Fullname:                   s.FullName,
		Tradesidetype:              s.TradeSideType,
		Ispalisade:                 false,
	}, nil
}
