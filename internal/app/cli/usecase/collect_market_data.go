package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICollectMarketData interface {
	Process(context.Context, bool) error
}

type CollectMarketData struct {
	api       *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewCollectMarketDataUsecase(api *webapi.MexcWebapi, stateRepo repo.IStateRepository) *CollectMarketData {
	return &CollectMarketData{api: api, stateRepo: stateRepo}
}

func (u *CollectMarketData) Process(ctx context.Context, debug bool) error {
	bookTickers, err := u.api.GetAllBookTickers(ctx)
	if err != nil {
		return wrap.Errorf("get all book tickers: %w", err)
	}
	dayTickers, err := u.api.GetAll24hTickers(ctx)
	if err != nil {
		return wrap.Errorf("get all 24h tickers: %w", err)
	}

	bookBySymbol := make(map[string]struct {
		bid, bidQty, ask, askQty float64
	}, len(*bookTickers))
	for _, ticker := range *bookTickers {
		bid, bidErr := strconv.ParseFloat(ticker.BidPrice, 64)
		bidQty, bidQtyErr := strconv.ParseFloat(ticker.BidQty, 64)
		ask, askErr := strconv.ParseFloat(ticker.AskPrice, 64)
		askQty, askQtyErr := strconv.ParseFloat(ticker.AskQty, 64)
		if bidErr != nil || bidQtyErr != nil || askErr != nil || askQtyErr != nil || bid <= 0 || ask <= 0 {
			continue
		}
		bookBySymbol[ticker.Symbol] = struct {
			bid, bidQty, ask, askQty float64
		}{bid, bidQty, ask, askQty}
	}

	collectedAt := time.Now().UTC()
	saved := 0
	for _, ticker := range *dayTickers {
		book, ok := bookBySymbol[ticker.Symbol]
		if !ok || book.ask <= book.bid {
			continue
		}
		if len(ticker.Symbol) < 5 || ticker.Symbol[len(ticker.Symbol)-4:] != "USDT" {
			continue
		}
		lastPrice, err := strconv.ParseFloat(ticker.LastPrice, 64)
		if err != nil || lastPrice <= 0 {
			continue
		}
		quoteVolume, err := strconv.ParseFloat(ticker.QuoteVolume, 64)
		if err != nil || quoteVolume < 0 {
			continue
		}
		change, err := strconv.ParseFloat(ticker.PriceChangePercent, 64)
		if err != nil {
			change = 0
		}

		if err := u.stateRepo.UpsertMarketSnapshot(ctx, repo.MarketSnapshot{
			Symbol:             ticker.Symbol,
			CollectedAt:        collectedAt,
			LastPrice:          lastPrice,
			BidPrice:           book.bid,
			BidQty:             book.bidQty,
			AskPrice:           book.ask,
			AskQty:             book.askQty,
			QuoteVolume24h:     quoteVolume,
			PriceChangePercent: change,
		}); err != nil {
			return wrap.Errorf("save market snapshot %s: %w", ticker.Symbol, err)
		}
		saved++
	}

	if debug {
		fmt.Printf("Сохранено market snapshots: %d\n", saved)
	}
	return nil
}
