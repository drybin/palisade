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
	"github.com/jackc/pgx/v5/pgtype"
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
		Support:      pgtype.Float8{Float64: support, Valid: true},
		Resistance:   pgtype.Float8{Float64: resistance, Valid: true},
		Rangevalue:   pgtype.Float8{Float64: rangeValue, Valid: true},
		Rangepercent: pgtype.Float8{Float64: rangePercent, Valid: true},
		Avgprice:     pgtype.Float8{Float64: avgPrice, Valid: true},
		Volatility:   pgtype.Float8{Float64: volatility, Valid: true},
		Maxdrawdown:  pgtype.Float8{Float64: maxDrawdown, Valid: true},
		Maxrise:      pgtype.Float8{Float64: maxRise, Valid: true},
		Symbol:       symbol,
	})

	if err != nil {
		return wrap.Errorf("failed to update palisade params for coin %s: %w", symbol, err)
	}

	return nil
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
	}, nil
}

// getLastCheck возвращает Lastcheck, если он не nil, иначе возвращает Date
func getLastCheck(c palisade_database.Coin) time.Time {
	if c.Lastcheck != nil {
		return *c.Lastcheck
	}
	return c.Date
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
