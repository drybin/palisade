package usecase

import (
	"strconv"

	"github.com/drybin/palisade/internal/domain/model/mexc"
)

// swapBookQuote — лучшие bid/ask по символу (исполнимые стороны книги).
type swapBookQuote struct {
	Bid float64
	Ask float64
}

// BuildSwapBookMap строит map symbol -> bid/ask из bookTicker (сторона 0 = нет данных).
func BuildSwapBookMap(rows *mexc.BookTickers) map[string]swapBookQuote {
	if rows == nil {
		return nil
	}
	out := make(map[string]swapBookQuote, len(*rows))
	for _, r := range *rows {
		if r.Symbol == "" {
			continue
		}
		var q swapBookQuote
		if b, err := strconv.ParseFloat(r.BidPrice, 64); err == nil && b > 0 {
			q.Bid = b
		}
		if a, err := strconv.ParseFloat(r.AskPrice, 64); err == nil && a > 0 {
			q.Ask = a
		}
		if q.Bid <= 0 && q.Ask <= 0 {
			continue
		}
		out[r.Symbol] = q
	}
	return out
}

// swapChainResult — результат расчёта цепочки USDT -> A -> B -> USDT (старт с 1 USDT).
// Цены — исполнимые: leg1 BUY по ask AUSDT, leg2 SELL по bid AB или BUY по ask BA, leg3 SELL по bid BUSDT.
type swapChainResult struct {
	priceAUSDT    float64
	priceBUSDT    float64
	symbolAB      string
	priceAB       float64
	amountA       float64
	amountB       float64
	amountUSDT    float64
	profitPercent float64
	usedDirectAB  bool
}

// calcSwapChainFromBook считает цепочку по лучшему bid/ask (агрессивные лимиты под немедленное исполнение).
func calcSwapChainFromBook(book map[string]swapBookQuote, coinA, coinB *mexc.SymbolDetail) (swapChainResult, bool) {
	var res swapChainResult
	symbolAUSDT := coinA.BaseAsset + "USDT"
	symbolBUSDT := coinB.BaseAsset + "USDT"

	aBook, ok := book[symbolAUSDT]
	if !ok || aBook.Ask <= 0 {
		return res, false
	}
	bBook, ok := book[symbolBUSDT]
	if !ok || bBook.Bid <= 0 {
		return res, false
	}

	res.priceAUSDT = aBook.Ask
	res.priceBUSDT = bBook.Bid

	amountA := 1.0 / res.priceAUSDT
	if amountA <= 0 {
		return res, false
	}
	res.amountA = amountA

	symbolAB := coinA.BaseAsset + coinB.BaseAsset
	symbolBA := coinB.BaseAsset + coinA.BaseAsset

	abBook, hasAB := book[symbolAB]
	baBook, hasBA := book[symbolBA]

	if hasAB && abBook.Bid > 0 {
		res.amountB = amountA * abBook.Bid
		res.symbolAB = symbolAB
		res.priceAB = abBook.Bid
		res.usedDirectAB = true
	} else if hasBA && baBook.Ask > 0 {
		res.amountB = amountA / baBook.Ask
		res.symbolAB = symbolBA
		res.priceAB = baBook.Ask
		res.usedDirectAB = false
	} else {
		return res, false
	}

	if res.amountB <= 0 {
		return res, false
	}

	res.amountUSDT = res.amountB * res.priceBUSDT
	res.profitPercent = (res.amountUSDT - 1.0) * 100.0
	return res, true
}
