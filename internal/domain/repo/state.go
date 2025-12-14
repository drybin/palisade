package repo

import (
	"context"
	"time"

	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
)

type GetCoinsParams struct {
	IsSpotTradingAllowed *bool
	IsPalisade           *bool
	Limit                int
	Offset               int
}

type IStateRepository interface {
	GetCoinState(context.Context, model.Coin, model.Coin) (*model.State, error)
	GetCountLogsByCoin(context.Context, model.Coin, model.Coin) (*int, error)
	SaveCoin(context.Context, *mexc.SymbolDetail) error
	GetCoinInfo(context.Context, string) (*mexc.SymbolDetail, error)
	GetCoins(context.Context, GetCoinsParams) ([]mexc.SymbolDetail, error)
	GetCoinsToProcess(context.Context, int, int) ([]mexc.SymbolDetail, error)
	UpdateIsPalisade(context.Context, string, bool) error
	UpdatePalisadeParams(context.Context, string, float64, float64, float64, float64, float64, float64, float64, float64) error
	GetNextTradeId(context.Context) (int, error)
	SaveTradeLog(context.Context, SaveTradeLogParams) (*TradeLog, error)
	UpdateDealDateTradeLog(context.Context, int, time.Time) error
	UpdateCancelDateTradeLog(context.Context, int, time.Time) error
	UpdateSuccesTradeLog(context.Context, int, time.Time, float64, float64) error
	GetOpenOrders(context.Context) ([]TradeLog, error)
}

type SaveTradeLogParams struct {
	OpenDate    time.Time
	OpenBalance float64
	Symbol      string
	BuyPrice    float64
	Amount      float64
	OrderId     string
	UpLevel     float64
	DownLevel   float64
}

type TradeLog struct {
	ID           int
	OpenDate     time.Time
	DealDate     *time.Time
	CloseDate    *time.Time
	CancelDate   *time.Time
	OpenBalance  float64
	CloseBalance float64
	Symbol       string
	BuyPrice     float64
	SellPrice    float64
	Amount       float64
	OrderId      string
	UpLevel      float64
	DownLevel    float64
}
