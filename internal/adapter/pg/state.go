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

func (u StateRepository) TryAcquireTradingLock(ctx context.Context, key string) (bool, error) {
	var acquired bool
	if err := u.Postgree.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtext($1)::bigint)", key).Scan(&acquired); err != nil {
		return false, wrap.Errorf("acquire trading lock %q: %w", key, err)
	}
	return acquired, nil
}

func (u StateRepository) ReleaseTradingLock(ctx context.Context, key string) error {
	var released bool
	if err := u.Postgree.QueryRow(ctx, "SELECT pg_advisory_unlock(hashtext($1)::bigint)", key).Scan(&released); err != nil {
		return wrap.Errorf("release trading lock %q: %w", key, err)
	}
	if !released {
		return wrap.Errorf("trading lock %q was not held by this connection", key)
	}
	return nil
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

	if maxId < 200 {
		maxId = 200
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

func (u StateRepository) UpdateTradeLevels(ctx context.Context, id int, upLevel, downLevel float64) error {
	db := palisade_database.New(u.Postgree)
	if err := db.UpdateTradeLevels(ctx, palisade_database.UpdateTradeLevelsParams{
		Uplevel:   upLevel,
		Downlevel: downLevel,
		ID:        id,
	}); err != nil {
		return wrap.Errorf("failed to update trade levels for id %d: %w", id, err)
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

func (u StateRepository) UpdateAmountTradeLog(ctx context.Context, id int, amount float64) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateAmountTradeLog(ctx, palisade_database.UpdateAmountTradeLogParams{
		ID:     id,
		Amount: amount,
	})
	if err != nil {
		return wrap.Errorf("failed to update amount for trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) UpdateTradeFill(ctx context.Context, id int, buyPrice, amount float64) error {
	db := palisade_database.New(u.Postgree)
	if err := db.UpdateTradeFill(ctx, palisade_database.UpdateTradeFillParams{
		BuyPrice: buyPrice,
		Amount:   amount,
		ID:       id,
	}); err != nil {
		return wrap.Errorf("failed to update trade fill for id %d: %w", id, err)
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

func mapTradeLogManualToDomainModel(t palisade_database.TradeLogManual) *repo.TradeLog {
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

func (u StateRepository) SaveTradeLogManual(ctx context.Context, params repo.SaveTradeLogParams) (*repo.TradeLog, error) {
	db := palisade_database.New(u.Postgree)
	tradeLog, err := db.SaveTradeLogManual(ctx, palisade_database.SaveTradeLogManualParams{
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
		return nil, wrap.Errorf("failed to save manual trade log: %w", err)
	}
	return mapTradeLogManualToDomainModel(tradeLog), nil
}

func (u StateRepository) GetOpenOrdersManual(ctx context.Context) ([]repo.TradeLog, error) {
	db := palisade_database.New(u.Postgree)
	tradeLogs, err := db.GetOpenOrdersManual(ctx)
	if err != nil {
		return nil, wrap.Errorf("failed to get open manual orders: %w", err)
	}
	result := make([]repo.TradeLog, 0, len(tradeLogs))
	for _, tl := range tradeLogs {
		result = append(result, *mapTradeLogManualToDomainModel(tl))
	}
	return result, nil
}

func (u StateRepository) GetTradeLogManualById(ctx context.Context, id int) (*repo.TradeLog, error) {
	db := palisade_database.New(u.Postgree)
	row, err := db.GetTradeLogManualById(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, wrap.Errorf("trade_log_manual id %d not found", id)
		}
		return nil, wrap.Errorf("failed to get trade_log_manual: %w", err)
	}
	return mapTradeLogManualToDomainModel(row), nil
}

func (u StateRepository) GetNextTradeIdManual(ctx context.Context) (int, error) {
	db := palisade_database.New(u.Postgree)
	lastTradeId, err := db.GetLastTradeIdManual(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 201, nil
		}
		return 0, wrap.Errorf("failed to get last manual trade id: %w", err)
	}
	if lastTradeId == nil {
		return 201, nil
	}
	var maxID int
	switch v := lastTradeId.(type) {
	case int64:
		maxID = int(v)
	case int32:
		maxID = int(v)
	case int:
		maxID = v
	case float64:
		maxID = int(v)
	default:
		maxIDStr := fmt.Sprintf("%v", lastTradeId)
		parsed, parseErr := strconv.Atoi(maxIDStr)
		if parseErr != nil {
			return 0, wrap.Errorf("failed to convert last manual trade id to int: %w", parseErr)
		}
		maxID = parsed
	}
	if maxID < 200 {
		maxID = 200
	}
	return maxID + 1, nil
}

func (u StateRepository) UpdateDealDateTradeLogManual(ctx context.Context, id int, dealDate time.Time) error {
	db := palisade_database.New(u.Postgree)
	err := db.UpdateDealDateTradeLogManual(ctx, palisade_database.UpdateDealDateTradeLogManualParams{
		ID:       id,
		DealDate: &dealDate,
	})
	if err != nil {
		return wrap.Errorf("failed to update deal date manual id %d: %w", id, err)
	}
	return nil
}

func (u StateRepository) UpdateCancelDateTradeLogManual(ctx context.Context, id int, cancelDate time.Time) error {
	db := palisade_database.New(u.Postgree)
	err := db.UpdateCancelDateTradeLogManual(ctx, palisade_database.UpdateCancelDateTradeLogManualParams{
		ID:         id,
		CancelDate: &cancelDate,
	})
	if err != nil {
		return wrap.Errorf("failed to update cancel date manual id %d: %w", id, err)
	}
	return nil
}

func (u StateRepository) UpdateSellOrderIdTradeLogManual(ctx context.Context, id int, sellOrderId string) error {
	db := palisade_database.New(u.Postgree)
	err := db.UpdateSellOrderIdTradeLogManual(ctx, palisade_database.UpdateSellOrderIdTradeLogManualParams{
		ID:          id,
		OrderidSell: &sellOrderId,
	})
	if err != nil {
		return wrap.Errorf("failed to update sell order id manual %d: %w", id, err)
	}
	return nil
}

func (u StateRepository) UpdateAmountTradeLogManual(ctx context.Context, id int, amount float64) error {
	db := palisade_database.New(u.Postgree)

	err := db.UpdateAmountTradeLogManual(ctx, palisade_database.UpdateAmountTradeLogManualParams{
		ID:     id,
		Amount: amount,
	})
	if err != nil {
		return wrap.Errorf("failed to update amount for manual trade log id %d: %w", id, err)
	}

	return nil
}

func (u StateRepository) UpdateSuccesTradeLogManual(ctx context.Context, id int, closeDate time.Time, closeBalance float64, sellPrice float64) error {
	db := palisade_database.New(u.Postgree)
	err := db.UpdateSuccesTradeLogManual(ctx, palisade_database.UpdateSuccesTradeLogManualParams{
		ID:           id,
		CloseDate:    &closeDate,
		CloseBalance: &closeBalance,
		SellPrice:    &sellPrice,
	})
	if err != nil {
		return wrap.Errorf("failed to update success manual trade id %d: %w", id, err)
	}
	return nil
}

func (u StateRepository) UpsertMarketSnapshot(ctx context.Context, snapshot repo.MarketSnapshot) error {
	db := palisade_database.New(u.Postgree)
	return db.UpsertMarketSnapshot(ctx, palisade_database.UpsertMarketSnapshotParams{
		Symbol:             snapshot.Symbol,
		CollectedAt:        snapshot.CollectedAt,
		LastPrice:          snapshot.LastPrice,
		BidPrice:           snapshot.BidPrice,
		BidQty:             snapshot.BidQty,
		AskPrice:           snapshot.AskPrice,
		AskQty:             snapshot.AskQty,
		QuoteVolume24h:     snapshot.QuoteVolume24h,
		PriceChangePercent: snapshot.PriceChangePercent,
	})
}

func (u StateRepository) ListMarketSnapshots(ctx context.Context) ([]repo.MarketSnapshot, error) {
	db := palisade_database.New(u.Postgree)
	rows, err := db.ListMarketSnapshots(ctx)
	if err != nil {
		return nil, wrap.Errorf("list market snapshots: %w", err)
	}
	result := make([]repo.MarketSnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, repo.MarketSnapshot{
			Symbol:             row.Symbol,
			CollectedAt:        row.CollectedAt,
			LastPrice:          row.LastPrice,
			BidPrice:           row.BidPrice,
			BidQty:             row.BidQty,
			AskPrice:           row.AskPrice,
			AskQty:             row.AskQty,
			QuoteVolume24h:     row.QuoteVolume24h,
			PriceChangePercent: row.PriceChangePercent,
		})
	}
	return result, nil
}

func (u StateRepository) GetLastPalisadeSignal(ctx context.Context, symbol string) (*time.Time, error) {
	db := palisade_database.New(u.Postgree)
	sentAt, err := db.GetLastPalisadeSignal(ctx, symbol)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("get last palisade signal for %s: %w", symbol, err)
	}
	return &sentAt, nil
}

func (u StateRepository) SavePalisadeSignal(ctx context.Context, symbol string, sentAt time.Time, score float64) error {
	db := palisade_database.New(u.Postgree)
	return db.SavePalisadeSignal(ctx, palisade_database.SavePalisadeSignalParams{
		Symbol: symbol,
		SentAt: sentAt,
		Score:  score,
	})
}

func (u StateRepository) SavePalisadeSignalState(ctx context.Context, signal repo.PalisadeSignalState) error {
	db := palisade_database.New(u.Postgree)
	return db.SavePalisadeSignalState(ctx, palisade_database.SavePalisadeSignalStateParams{
		Symbol:             signal.Symbol,
		StrategyVersion:    signal.StrategyVersion,
		SupportPrice:       signal.SupportPrice,
		EntryPrice:         signal.EntryPrice,
		TargetPrice:        signal.TargetPrice,
		MinExitPrice:       signal.MinExitPrice,
		NetProfit:          signal.NetProfit,
		Score:              float64(signal.Score),
		Status:             signal.Status,
		InvalidationReason: signal.InvalidationReason,
		ValidUntil:         signal.ValidUntil,
		UpdatedAt:          signal.UpdatedAt,
	})
}

func (u StateRepository) ListActivePalisadeSignals(ctx context.Context) ([]repo.PalisadeSignalState, error) {
	db := palisade_database.New(u.Postgree)
	rows, err := db.ListActivePalisadeSignals(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []repo.PalisadeSignalState{}, nil
		}
		return nil, wrap.Errorf("list active palisade signals: %w", err)
	}
	result := make([]repo.PalisadeSignalState, 0, len(rows))
	for _, row := range rows {
		result = append(result, repo.PalisadeSignalState{
			Symbol:             row.Symbol,
			SentAt:             row.SentAt,
			StrategyVersion:    int(row.StrategyVersion),
			SupportPrice:       row.SupportPrice,
			EntryPrice:         row.EntryPrice,
			TargetPrice:        row.TargetPrice,
			MinExitPrice:       row.MinExitPrice,
			NetProfit:          row.NetProfit,
			Score:              int(row.Score),
			Status:             row.Status,
			InvalidationReason: row.InvalidationReason,
			ValidUntil:         row.ValidUntil,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	return result, nil
}

func (u StateRepository) CreateOrderIntent(ctx context.Context, intent repo.OrderIntent) (*repo.OrderIntent, error) {
	db := palisade_database.New(u.Postgree)
	tradeID := pgtype.Int4{}
	if intent.TradeID > 0 {
		tradeID = pgtype.Int4{Int32: int32(intent.TradeID), Valid: true}
	}
	created, err := db.CreateOrderIntent(ctx, palisade_database.CreateOrderIntentParams{
		ClientOrderID:      intent.ClientOrderID,
		Symbol:             intent.Symbol,
		Side:               intent.Side,
		Price:              intent.Price,
		Quantity:           intent.Quantity,
		OpenBalance:        intent.OpenBalance,
		TargetPrice:        intent.TargetPrice,
		Status:             intent.Status,
		ExchangeOrderID:    intent.ExchangeOrderID,
		TradeID:            tradeID,
		ExecutedQuantity:   intent.ExecutedQuantity,
		CumulativeQuoteQty: intent.CumulativeQuoteQty,
		LastError:          intent.LastError,
		CreatedAt:          intent.CreatedAt,
		UpdatedAt:          intent.UpdatedAt,
	})
	if err != nil {
		return nil, wrap.Errorf("create order intent %s: %w", intent.ClientOrderID, err)
	}
	return mapOrderIntentToDomain(created), nil
}

func (u StateRepository) UpdateOrderIntent(ctx context.Context, id int, status, exchangeOrderID string, executed, quote float64, lastError string) error {
	db := palisade_database.New(u.Postgree)
	if err := db.UpdateOrderIntent(ctx, palisade_database.UpdateOrderIntentParams{
		ID:                 id,
		Status:             status,
		ExchangeOrderID:    exchangeOrderID,
		ExecutedQuantity:   executed,
		CumulativeQuoteQty: quote,
		LastError:          lastError,
	}); err != nil {
		return wrap.Errorf("update order intent %d: %w", id, err)
	}
	return nil
}

func (u StateRepository) UpdateOrderIntentTradeID(ctx context.Context, id, tradeID int) error {
	db := palisade_database.New(u.Postgree)
	value := pgtype.Int4{Int32: int32(tradeID), Valid: tradeID > 0}
	if err := db.UpdateOrderIntentTradeID(ctx, palisade_database.UpdateOrderIntentTradeIDParams{ID: id, TradeID: value}); err != nil {
		return wrap.Errorf("update order intent %d trade id: %w", id, err)
	}
	return nil
}

func (u StateRepository) ListRecoverableOrderIntents(ctx context.Context) ([]repo.OrderIntent, error) {
	db := palisade_database.New(u.Postgree)
	rows, err := db.ListRecoverableOrderIntents(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []repo.OrderIntent{}, nil
		}
		return nil, wrap.Errorf("list recoverable order intents: %w", err)
	}
	result := make([]repo.OrderIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapOrderIntentToDomain(row))
	}
	return result, nil
}

func (u StateRepository) ListOrderIntentsByTradeID(ctx context.Context, tradeID int) ([]repo.OrderIntent, error) {
	db := palisade_database.New(u.Postgree)
	rows, err := db.ListOrderIntentsByTradeID(ctx, pgtype.Int4{Int32: int32(tradeID), Valid: tradeID > 0})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []repo.OrderIntent{}, nil
		}
		return nil, wrap.Errorf("list order intents for trade %d: %w", tradeID, err)
	}
	result := make([]repo.OrderIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapOrderIntentToDomain(row))
	}
	return result, nil
}

func mapOrderIntentToDomain(row palisade_database.PalisadeOrderIntent) *repo.OrderIntent {
	tradeID := 0
	if row.TradeID.Valid {
		tradeID = int(row.TradeID.Int32)
	}
	return &repo.OrderIntent{
		ID:                 row.ID,
		ClientOrderID:      row.ClientOrderID,
		Symbol:             row.Symbol,
		Side:               row.Side,
		Price:              row.Price,
		Quantity:           row.Quantity,
		OpenBalance:        row.OpenBalance,
		TargetPrice:        row.TargetPrice,
		Status:             row.Status,
		ExchangeOrderID:    row.ExchangeOrderID,
		TradeID:            tradeID,
		ExecutedQuantity:   row.ExecutedQuantity,
		CumulativeQuoteQty: row.CumulativeQuoteQty,
		LastError:          row.LastError,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func (u StateRepository) GetOpenPaperTradeBySymbol(ctx context.Context, symbol string, strategyVersion int) (*repo.PaperTrade, error) {
	db := palisade_database.New(u.Postgree)
	row, err := db.GetOpenPaperTradeBySymbol(ctx, palisade_database.GetOpenPaperTradeBySymbolParams{
		Symbol:          symbol,
		StrategyVersion: strategyVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("get open paper trade for %s: %w", symbol, err)
	}
	return mapPaperTradeToDomain(row), nil
}

func (u StateRepository) GetPaperTradeBySignal(ctx context.Context, symbol string, signalAt time.Time, strategyVersion int) (*repo.PaperTrade, error) {
	db := palisade_database.New(u.Postgree)
	row, err := db.GetPaperTradeBySignal(ctx, palisade_database.GetPaperTradeBySignalParams{
		Symbol:          symbol,
		SignalAt:        signalAt,
		StrategyVersion: strategyVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap.Errorf("get paper trade for %s signal %s: %w", symbol, signalAt.Format(time.RFC3339), err)
	}
	return mapPaperTradeToDomain(row), nil
}

func (u StateRepository) ListOpenPaperTrades(ctx context.Context, strategyVersion int) ([]repo.PaperTrade, error) {
	db := palisade_database.New(u.Postgree)
	rows, err := db.ListOpenPaperTrades(ctx, strategyVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []repo.PaperTrade{}, nil
		}
		return nil, wrap.Errorf("list open paper trades: %w", err)
	}
	result := make([]repo.PaperTrade, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapPaperTradeToDomain(row))
	}
	return result, nil
}

func (u StateRepository) CreatePaperTrade(ctx context.Context, trade repo.PaperTrade) (*repo.PaperTrade, error) {
	db := palisade_database.New(u.Postgree)
	row, err := db.CreatePaperTrade(ctx, palisade_database.CreatePaperTradeParams{
		StrategyVersion:    trade.StrategyVersion,
		Symbol:             trade.Symbol,
		SignalAt:           trade.SignalAt,
		Status:             trade.Status,
		EntryMode:          trade.EntryMode,
		SupportPrice:       trade.SupportPrice,
		EntryPrice:         trade.EntryPrice,
		TargetPrice:        trade.TargetPrice,
		MinExitPrice:       trade.MinExitPrice,
		ExpectedNetProfit:  trade.ExpectedNetProfit,
		BreakEvenArmed:     trade.BreakEvenArmed,
		MaxBidPrice:        trade.MaxBidPrice,
		MinBidPrice:        trade.MinBidPrice,
		EntryLowPrice:      trade.EntryLowPrice,
		PartialProfitTaken: trade.PartialProfitTaken,
		Quantity:           trade.Quantity,
		FilledQuantity:     trade.FilledQuantity,
		SoldQuantity:       trade.SoldQuantity,
		BuyQuote:           trade.BuyQuote,
		SellQuote:          trade.SellQuote,
		Fees:               trade.Fees,
		Pnl:                trade.PnL,
		OpenedAt:           trade.OpenedAt,
		ClosedAt:           trade.ClosedAt,
		ExitReason:         trade.ExitReason,
		LastPrice:          trade.LastPrice,
		UpdatedAt:          trade.UpdatedAt,
	})
	if err != nil {
		return nil, wrap.Errorf("create paper trade %s: %w", trade.Symbol, err)
	}
	return mapPaperTradeToDomain(row), nil
}

func (u StateRepository) UpdatePaperTrade(ctx context.Context, trade repo.PaperTrade) error {
	db := palisade_database.New(u.Postgree)
	if err := db.UpdatePaperTrade(ctx, palisade_database.UpdatePaperTradeParams{
		ID:                 trade.ID,
		Status:             trade.Status,
		TargetPrice:        trade.TargetPrice,
		BreakEvenArmed:     trade.BreakEvenArmed,
		MaxBidPrice:        trade.MaxBidPrice,
		MinBidPrice:        trade.MinBidPrice,
		EntryLowPrice:      trade.EntryLowPrice,
		PartialProfitTaken: trade.PartialProfitTaken,
		FilledQuantity:     trade.FilledQuantity,
		SoldQuantity:       trade.SoldQuantity,
		BuyQuote:           trade.BuyQuote,
		SellQuote:          trade.SellQuote,
		Fees:               trade.Fees,
		Pnl:                trade.PnL,
		OpenedAt:           trade.OpenedAt,
		ClosedAt:           trade.ClosedAt,
		ExitReason:         trade.ExitReason,
		LastPrice:          trade.LastPrice,
		UpdatedAt:          trade.UpdatedAt,
	}); err != nil {
		return wrap.Errorf("update paper trade %d: %w", trade.ID, err)
	}
	return nil
}

func (u StateRepository) GetPaperTradeStats(ctx context.Context, strategyVersion int) (repo.PaperTradeStats, error) {
	db := palisade_database.New(u.Postgree)
	row, err := db.GetPaperTradeStats(ctx, strategyVersion)
	if err != nil {
		return repo.PaperTradeStats{}, wrap.Errorf("get paper trade stats: %w", err)
	}
	return repo.PaperTradeStats{
		Total:      row.Total,
		Closed:     row.Closed,
		Canceled:   row.Canceled,
		Open:       row.Open,
		TotalPnL:   row.TotalPnl,
		OpenPnL:    row.OpenPnl,
		AveragePnL: row.AveragePnl,
		Wins:       row.Wins,
		Losses:     row.Losses,
	}, nil
}

func mapPaperTradeToDomain(row palisade_database.PaperTrade) *repo.PaperTrade {
	return &repo.PaperTrade{
		ID:                 row.ID,
		StrategyVersion:    row.StrategyVersion,
		Symbol:             row.Symbol,
		SignalAt:           row.SignalAt,
		Status:             row.Status,
		EntryMode:          row.EntryMode,
		SupportPrice:       row.SupportPrice,
		EntryPrice:         row.EntryPrice,
		TargetPrice:        row.TargetPrice,
		MinExitPrice:       row.MinExitPrice,
		ExpectedNetProfit:  row.ExpectedNetProfit,
		BreakEvenArmed:     row.BreakEvenArmed,
		MaxBidPrice:        row.MaxBidPrice,
		MinBidPrice:        row.MinBidPrice,
		EntryLowPrice:      row.EntryLowPrice,
		PartialProfitTaken: row.PartialProfitTaken,
		Quantity:           row.Quantity,
		FilledQuantity:     row.FilledQuantity,
		SoldQuantity:       row.SoldQuantity,
		BuyQuote:           row.BuyQuote,
		SellQuote:          row.SellQuote,
		Fees:               row.Fees,
		PnL:                row.Pnl,
		OpenedAt:           row.OpenedAt,
		ClosedAt:           row.ClosedAt,
		ExitReason:         row.ExitReason,
		LastPrice:          row.LastPrice,
		UpdatedAt:          row.UpdatedAt,
	}
}

func mapCoinToDomainModel(c palisade_database.Coin) (*mexc.SymbolDetail, error) {
	return &mexc.SymbolDetail{
		Symbol:                     c.Symbol,
		Status:                     strconv.Itoa(c.Status),
		BaseAsset:                  c.Baseasset,
		BaseAssetPrecision:         c.Baseassetprecision,
		QuoteAsset:                 c.Quoteasset,
		QuotePrecision:             c.Quoteprecision,
		QuoteAssetPrecision:        c.Quoteassetprecision,
		BaseCommissionPrecision:    c.Basecommissionprecision,
		QuoteCommissionPrecision:   c.Quotecommissionprecision,
		OrderTypes:                 c.Ordertypes,
		IsSpotTradingAllowed:       c.Isspottradingallowed,
		IsMarginTradingAllowed:     c.Ismargintradingallowed,
		QuoteAmountPrecision:       fmt.Sprintf("%f", c.Quoteamountprecision),
		BaseSizePrecision:          strconv.FormatFloat(c.Basesizeprecision, 'f', -1, 64),
		Permissions:                c.Permissions,
		Filters:                    []mexc.SymbolFilter{},
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
