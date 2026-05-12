package usecase

import (
	"math"
	"strconv"
	"strings"

	"github.com/drybin/palisade/internal/domain/model/mexc"
)

// swapDefaultTakerFee — если в exchangeInfo нет takerCommission (0.1% типичный спот).
const swapDefaultTakerFee = 0.001

// swapChainDefaultInterBuffer подставляется при meta.InterBuffer <= 0 (как swapIntermediateBuffer в исполнении).
const swapChainDefaultInterBuffer = 0.999

// SwapChainMeta задаёт индекс exchangeInfo по символам; из него берутся шаг лота и taker для трёх пар цепочки.
// Если nil или в Index нет одной из пар (AUSDT, BUSDT, AB после роутинга), используется непрерывная модель.
type SwapChainMeta struct {
	Index       map[string]*mexc.SymbolDetail
	InterBuffer float64
}

// swapTakerFeeRate парсит TakerCommission символа (доля 0…1); при ошибке — swapDefaultTakerFee.
func swapTakerFeeRate(sym *mexc.SymbolDetail) float64 {
	if sym == nil {
		return swapDefaultTakerFee
	}
	raw := strings.TrimSpace(sym.TakerCommission)
	if raw == "" {
		return swapDefaultTakerFee
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f < 0 || f >= 0.5 {
		return swapDefaultTakerFee
	}
	return f
}

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

func calcSwapChainFromBookContinuous(book map[string]swapBookQuote, coinA, coinB *mexc.SymbolDetail) (swapChainResult, bool) {
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

// calcSwapChainFromBook считает цепочку по лучшему bid/ask.
// meta == nil или неполный Index → непрерывная модель (как раньше без шагов и комиссий).
// Иначе — округление по лотам, swapIntermediateBuffer и taker по полям символов.
func calcSwapChainFromBook(book map[string]swapBookQuote, coinA, coinB *mexc.SymbolDetail, meta *SwapChainMeta) (swapChainResult, bool) {
	res, ok := calcSwapChainFromBookContinuous(book, coinA, coinB)
	if !ok {
		return swapChainResult{}, false
	}
	if meta == nil || meta.Index == nil {
		return res, true
	}

	symbolAUSDT := coinA.BaseAsset + "USDT"
	symbolBUSDT := coinB.BaseAsset + "USDT"
	sa := meta.Index[symbolAUSDT]
	sb := meta.Index[symbolBUSDT]
	sab := meta.Index[res.symbolAB]
	if sa == nil || sb == nil || sab == nil {
		return res, true
	}

	stepA, e1 := swapLotStep(sa)
	stepB, e2 := swapLotStep(sb)
	stepAB, e3 := swapLotStep(sab)
	if e1 != nil || e2 != nil || e3 != nil {
		return res, true
	}

	buf := meta.InterBuffer
	if buf <= 0 {
		buf = swapChainDefaultInterBuffer
	}

	fA := swapTakerFeeRate(sa)
	fAB := swapTakerFeeRate(sab)
	fB := swapTakerFeeRate(sb)

	real, applied := applySwapChainRealism(res, stepA, stepAB, stepB, buf, fA, fAB, fB)
	if applied {
		return real, true
	}
	return res, true
}

// applySwapChainRealism применяет шаги лота и комиссии к уже выбранному маршруту res.
func applySwapChainRealism(
	res swapChainResult,
	stepA, stepAB, stepB float64,
	interBuffer float64,
	feeAUSDT, feeAB, feeBUSDT float64,
) (swapChainResult, bool) {
	if stepA <= 0 || stepAB <= 0 || stepB <= 0 || interBuffer <= 0 {
		return swapChainResult{}, false
	}

	askA := res.priceAUSDT
	bidB := res.priceBUSDT
	priceAB := res.priceAB
	direct := res.usedDirectAB

	qty1 := swapRoundQtyDown(1.0/askA, stepA)
	if qty1 <= 0 {
		return swapChainResult{}, false
	}
	costUSDT := qty1 * askA
	if costUSDT <= 0 {
		return swapChainResult{}, false
	}

	qtyANet := qty1 * (1.0 - feeAUSDT)
	if qtyANet <= 0 {
		return swapChainResult{}, false
	}

	qtyAExpected := qtyANet * interBuffer
	var qty2 float64
	if direct {
		qty2 = swapRoundQtyDown(math.Min(qtyANet, qtyAExpected), stepAB)
	} else {
		planB := qtyANet / priceAB
		maxAffordB := qtyAExpected / priceAB
		qty2 = swapRoundQtyDown(math.Min(planB, maxAffordB), stepAB)
	}
	if qty2 <= 0 {
		return swapChainResult{}, false
	}

	var amountB float64
	if direct {
		amountB = qty2 * priceAB * (1.0 - feeAB)
	} else {
		amountB = qty2 * (1.0 - feeAB)
	}
	qty3 := swapRoundQtyDown(amountB, stepB)
	if qty3 <= 0 {
		return swapChainResult{}, false
	}
	outUSDT := qty3 * bidB * (1.0 - feeBUSDT)

	out := res
	out.amountA = qty1
	out.amountB = qty3
	out.amountUSDT = outUSDT
	out.profitPercent = (outUSDT/costUSDT - 1.0) * 100.0
	return out, true
}

// BuildSymbolDetailIndex индекс symbol -> детали пары из ответа exchangeInfo.
func BuildSymbolDetailIndex(info *mexc.SymbolInfo) map[string]*mexc.SymbolDetail {
	if info == nil {
		return nil
	}
	out := make(map[string]*mexc.SymbolDetail, len(info.Symbols))
	for i := range info.Symbols {
		s := &info.Symbols[i]
		out[s.Symbol] = s
	}
	return out
}
