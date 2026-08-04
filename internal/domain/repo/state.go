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

type MarketSnapshot struct {
	Symbol             string
	CollectedAt        time.Time
	LastPrice          float64
	BidPrice           float64
	BidQty             float64
	AskPrice           float64
	AskQty             float64
	QuoteVolume24h     float64
	PriceChangePercent float64
}

type PalisadeSignalState struct {
	Symbol             string
	SentAt             time.Time
	StrategyVersion    int
	SupportPrice       float64
	EntryPrice         float64
	TargetPrice        float64
	MinExitPrice       float64
	NetProfit          float64
	Score              int
	Status             string
	InvalidationReason string
	ValidUntil         time.Time
	UpdatedAt          time.Time
}

type OrderIntent struct {
	ID                 int
	ClientOrderID      string
	Symbol             string
	Side               string
	Price              float64
	Quantity           float64
	OpenBalance        float64
	TargetPrice        float64
	Status             string
	ExchangeOrderID    string
	TradeID            int
	ExecutedQuantity   float64
	CumulativeQuoteQty float64
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PaperTrade struct {
	ID                int
	StrategyVersion   int
	Symbol            string
	SignalAt          time.Time
	Status            string
	EntryMode         string
	SupportPrice      float64
	EntryPrice        float64
	TargetPrice       float64
	MinExitPrice      float64
	ExpectedNetProfit float64
	Quantity          float64
	FilledQuantity    float64
	SoldQuantity      float64
	BuyQuote          float64
	SellQuote         float64
	Fees              float64
	PnL               float64
	OpenedAt          *time.Time
	ClosedAt          *time.Time
	ExitReason        string
	LastPrice         float64
	UpdatedAt         time.Time
}

type PaperTradeStats struct {
	Total      int
	Closed     int
	Canceled   int
	Open       int
	TotalPnL   float64
	OpenPnL    float64
	AveragePnL float64
	Wins       int
	Losses     int
}

type IStateRepository interface {
	TryAcquireTradingLock(context.Context, string) (bool, error)
	ReleaseTradingLock(context.Context, string) error
	GetCoinState(context.Context, model.Coin, model.Coin) (*model.State, error)
	GetCountLogsByCoin(context.Context, model.Coin, model.Coin) (*int, error)
	SaveCoin(context.Context, *mexc.SymbolDetail) error
	GetCoinInfo(context.Context, string) (*mexc.SymbolDetail, error)
	GetCoins(context.Context, GetCoinsParams) ([]mexc.SymbolDetail, error)
	GetCoinsToProcess(context.Context, int, int) ([]mexc.SymbolDetail, error)
	GetCoinsToProcessTPTU(context.Context, int, int) ([]mexc.SymbolDetail, error)
	UpdateIsPalisade(context.Context, string, bool) error
	UpdatePalisadeParams(context.Context, string, float64, float64, float64, float64, float64, float64, float64, float64) error
	GetNextTradeId(context.Context) (int, error)
	SaveTradeLog(context.Context, SaveTradeLogParams) (*TradeLog, error)
	UpdateTradeLevels(context.Context, int, float64, float64) error
	UpdateTradeFill(context.Context, int, float64, float64) error
	UpdateDealDateTradeLog(context.Context, int, time.Time) error
	UpdateCancelDateTradeLog(context.Context, int, time.Time) error
	UpdateAmountTradeLog(context.Context, int, float64) error
	UpdateSellOrderIdTradeLog(context.Context, int, string) error
	UpdateSuccesTradeLog(context.Context, int, time.Time, float64, float64) error
	GetOpenOrders(context.Context) ([]TradeLog, error)

	SaveTradeLogManual(context.Context, SaveTradeLogParams) (*TradeLog, error)
	GetOpenOrdersManual(context.Context) ([]TradeLog, error)
	GetTradeLogManualById(context.Context, int) (*TradeLog, error)
	GetNextTradeIdManual(context.Context) (int, error)
	UpdateDealDateTradeLogManual(context.Context, int, time.Time) error
	UpdateCancelDateTradeLogManual(context.Context, int, time.Time) error
	UpdateAmountTradeLogManual(context.Context, int, float64) error
	UpdateSellOrderIdTradeLogManual(context.Context, int, string) error
	UpdateSuccesTradeLogManual(context.Context, int, time.Time, float64, float64) error
	UpsertMarketSnapshot(context.Context, MarketSnapshot) error
	ListMarketSnapshots(context.Context) ([]MarketSnapshot, error)
	GetLastPalisadeSignal(context.Context, string) (*time.Time, error)
	SavePalisadeSignal(context.Context, string, time.Time, float64) error
	SavePalisadeSignalState(context.Context, PalisadeSignalState) error
	ListActivePalisadeSignals(context.Context) ([]PalisadeSignalState, error)
	CreateOrderIntent(context.Context, OrderIntent) (*OrderIntent, error)
	UpdateOrderIntent(context.Context, int, string, string, float64, float64, string) error
	UpdateOrderIntentTradeID(context.Context, int, int) error
	ListRecoverableOrderIntents(context.Context) ([]OrderIntent, error)
	ListOrderIntentsByTradeID(context.Context, int) ([]OrderIntent, error)
	GetOpenPaperTradeBySymbol(context.Context, string, int) (*PaperTrade, error)
	GetPaperTradeBySignal(context.Context, string, time.Time, int) (*PaperTrade, error)
	ListOpenPaperTrades(context.Context, int) ([]PaperTrade, error)
	CreatePaperTrade(context.Context, PaperTrade) (*PaperTrade, error)
	UpdatePaperTrade(context.Context, PaperTrade) error
	GetPaperTradeStats(context.Context, int) (PaperTradeStats, error)
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
	OrderId_sell string
	UpLevel      float64
	DownLevel    float64
}
