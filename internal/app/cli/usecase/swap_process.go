package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/helpers"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

const (
	minProfitPercent     = 1.0
	amountInUSDT         = 10.0
	orderFillWaitTimeout = 60 * time.Second
	orderFillPollInterval = 3 * time.Second
)

type ISwapProcess interface {
	Process(ctx context.Context) error
}

type SwapProcess struct {
	repo      *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewSwapProcessUsecase(repo *webapi.MexcWebapi, stateRepo repo.IStateRepository) *SwapProcess {
	return &SwapProcess{repo: repo, stateRepo: stateRepo}
}

func (u *SwapProcess) Process(ctx context.Context) error {
	fmt.Println("=== swap_process: поиск выгодной связки и исполнение лимитными ордерами ===\n")

	// --- 1. Получаем тикеры и строим карту цен ---
	fmt.Printf("[DEBUG] Загрузка тикеров...\n")
	tickers, err := u.repo.GetAllTickerPrices(ctx)
	if err != nil {
		return wrap.Errorf("failed to get ticker prices: %w", err)
	}
	priceMap := make(map[string]float64, len(*tickers))
	for _, t := range *tickers {
		p, err := strconv.ParseFloat(t.Price, 64)
		if err != nil || p <= 0 {
			continue
		}
		priceMap[t.Symbol] = p
	}
	fmt.Printf("[DEBUG] Загружено тикеров с ценой: %d\n\n", len(priceMap))

	// --- Список символов, по которым разрешена спот-торговля (из БД) ---
	spotTradingAllowed := true
	coinsAllowed, err := u.stateRepo.GetCoins(ctx, repo.GetCoinsParams{
		IsSpotTradingAllowed: &spotTradingAllowed,
		Limit:                10000,
		Offset:               0,
	})
	if err != nil {
		return wrap.Errorf("failed to get coins (spot allowed): %w", err)
	}
	symbolsAllowed := make(map[string]bool, len(coinsAllowed))
	for _, c := range coinsAllowed {
		symbolsAllowed[c.Symbol] = true
	}
	fmt.Printf("[DEBUG] В БД символов с IsSpotTradingAllowed: %d\n\n", len(symbolsAllowed))

	// --- 2. Строим все цепочки USDT -> A -> B -> USDT (через BTC/ETH/USDC), только если все 3 пары в списке разрешённых ---
	allowedHubs := map[string]bool{"BTC": true, "ETH": true, "USDC": true}
	var baseAssets []string
	for symbol := range priceMap {
		if strings.HasSuffix(symbol, "USDT") && len(symbol) > 4 {
			base := symbol[:len(symbol)-4]
			baseAssets = append(baseAssets, base)
		}
	}
	sort.Strings(baseAssets)

	type chainProfit struct {
		baseA, baseB string
		profit       float64
	}
	var allChains []chainProfit
	for i := 0; i < len(baseAssets); i++ {
		for j := 0; j < len(baseAssets); j++ {
			if i == j {
				continue
			}
			baseA, baseB := baseAssets[i], baseAssets[j]
			if !allowedHubs[baseA] && !allowedHubs[baseB] {
				continue
			}
			symbolAUSDT := baseA + "USDT"
			symbolBUSDT := baseB + "USDT"
			if !symbolsAllowed[symbolAUSDT] || !symbolsAllowed[symbolBUSDT] {
				continue
			}
			coinA := mexc.SymbolDetail{BaseAsset: baseA, Symbol: symbolAUSDT, QuoteAsset: "USDT"}
			coinB := mexc.SymbolDetail{BaseAsset: baseB, Symbol: symbolBUSDT, QuoteAsset: "USDT"}
			res, ok := u.calcChainProfit(priceMap, &coinA, &coinB)
			if !ok {
				continue
			}
			if !symbolsAllowed[res.symbolAB] {
				continue
			}
			allChains = append(allChains, chainProfit{baseA, baseB, res.profitPercent})
		}
	}
	sort.Slice(allChains, func(i, j int) bool { return allChains[i].profit > allChains[j].profit })

	if len(allChains) == 0 {
		fmt.Println("[DEBUG] Нет ни одной цепочки: все пары в тикерах или не все три символа (AUSDT, BUSDT, A-B) разрешены для спот-API в БД. Выход.")
		return nil
	}

	best := allChains[0]
	fmt.Printf("[DEBUG] Топ-5 связок по прибыли:\n")
	for i := 0; i < 5 && i < len(allChains); i++ {
		c := allChains[i]
		marker := ""
		if i == 0 {
			marker = "  <-- выбрана"
		}
		fmt.Printf("  %d. USDT -> %s -> %s -> USDT  |  %.4f%%%s\n", i+1, c.baseA, c.baseB, c.profit, marker)
	}
	fmt.Println()

	if best.profit <= minProfitPercent {
		fmt.Printf("[DEBUG] Лучшая связка %.4f%% не превышает порог %.1f%%. Выход без сделок.\n", best.profit, minProfitPercent)
		return nil
	}

	coinA := mexc.SymbolDetail{BaseAsset: best.baseA, Symbol: best.baseA + "USDT", QuoteAsset: "USDT"}
	coinB := mexc.SymbolDetail{BaseAsset: best.baseB, Symbol: best.baseB + "USDT", QuoteAsset: "USDT"}
	chain, ok := u.calcChainProfit(priceMap, &coinA, &coinB)
	if !ok {
		return wrap.Errorf("chain %s -> %s recalc failed", best.baseA, best.baseB)
	}

	// Масштабируем под amountInUSDT
	chain.amountA *= amountInUSDT
	chain.amountB *= amountInUSDT
	chain.amountUSDT *= amountInUSDT

	fmt.Printf("--- Выполняем цепочку: USDT -> %s -> %s -> USDT (прибыль %.4f%%, объём %.1f USDT) ---\n\n",
		best.baseA, best.baseB, best.profit, amountInUSDT)

	// --- 3. Проверка баланса USDT ---
	accountInfo, err := u.repo.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("failed to get balance: %w", err)
	}
	usdtBal, err := helpers.FindUSDTBalance(accountInfo.Balances)
	if err != nil {
		return wrap.Errorf("USDT balance not found: %w", err)
	}
	fmt.Printf("[DEBUG] Баланс USDT: Free=%.2f Locked=%.2f\n", usdtBal.Free, usdtBal.Locked)
	if usdtBal.Free < amountInUSDT {
		return wrap.Errorf("недостаточно USDT: нужно %.1f, свободно %.2f", amountInUSDT, usdtBal.Free)
	}

	// --- 4. Symbol info для пар (precision, min notional) ---
	symbolA := best.baseA + "USDT"
	symbolB := best.baseB + "USDT"
	infoA, err := u.repo.GetSymbolInfo(ctx, symbolA)
	if err != nil {
		return wrap.Errorf("get symbol info %s: %w", symbolA, err)
	}
	infoB, err := u.repo.GetSymbolInfo(ctx, symbolB)
	if err != nil {
		return wrap.Errorf("get symbol info %s: %w", symbolB, err)
	}
	infoAB, err := u.repo.GetSymbolInfo(ctx, chain.symbolAB)
	if err != nil {
		return wrap.Errorf("get symbol info %s: %w", chain.symbolAB, err)
	}

	symA := u.findSymbolDetail(infoA, symbolA)
	symB := u.findSymbolDetail(infoB, symbolB)
	symAB := u.findSymbolDetail(infoAB, chain.symbolAB)
	if symA == nil || symB == nil || symAB == nil {
		return wrap.Errorf("symbol not found in exchange info")
	}

	precisionA, _ := strconv.ParseFloat(symA.BaseSizePrecision, 64)
	precisionB, _ := strconv.ParseFloat(symB.BaseSizePrecision, 64)
	precisionAB, _ := strconv.ParseFloat(symAB.BaseSizePrecision, 64)

	roundQty := func(qty, precision float64) float64 {
		if precision <= 0 {
			return math.Floor(qty)
		}
		return math.Floor(qty/precision) * precision
	}

	qty1 := roundQty(amountInUSDT/chain.priceAUSDT, precisionA)
	price1 := chain.priceAUSDT

	fmt.Printf("[DEBUG] Шаг 1: BUY %s | quantity=%.8f price=%.8f (quote≈%.2f USDT)\n", symbolA, qty1, price1, qty1*price1)

	runID := time.Now().UnixMilli()
	// --- Шаг 1: Лимитный ордер BUY A за USDT ---
	clientOrderId1 := fmt.Sprintf("swap_%d_1_%s", runID, best.baseA)

	order1, err := u.repo.NewOrder(model.OrderParams{
		Symbol:           symbolA,
		Side:             order.BUY,
		OrderType:        order.LIMIT,
		Quantity:         qty1,
		Price:            price1,
		NewClientOrderId: clientOrderId1,
	})
	if err != nil {
		return wrap.Errorf("place order 1 (BUY %s): %w", symbolA, err)
	}
	fmt.Printf("[DEBUG] Ордер 1 размещён: orderId=%s\n", order1.OrderID)

	if err := u.waitOrderFilled(ctx, symbolA, order1.OrderID, "1"); err != nil {
		return err
	}

	// --- Шаг 2: Лимитный ордер на паре A-B (используем фактически купленное qty1) ---
	// usedDirectAB: пара AB (base=A, quote=B) — продаём A → SELL, quantity=qty1
	// иначе пара BA (base=B, quote=A) — покупаем B за A → BUY, quantity = qty1/priceBA
	var qty2 float64
	price2 := chain.priceAB
	if chain.usedDirectAB {
		qty2 = roundQty(qty1, precisionAB)
	} else {
		amountBFromA := qty1 / chain.priceAB
		qty2 = roundQty(amountBFromA, precisionAB)
	}

	clientOrderId2 := fmt.Sprintf("swap_%d_2_%s", runID, chain.symbolAB)

	var side2 order.Side
	if chain.usedDirectAB {
		side2 = order.SELL
	} else {
		side2 = order.BUY
	}
	fmt.Printf("[DEBUG] Шаг 2: %s %s | quantity=%.8f price=%.8f\n", side2.String(), chain.symbolAB, qty2, price2)

	order2, err := u.repo.NewOrder(model.OrderParams{
		Symbol:           chain.symbolAB,
		Side:             side2,
		OrderType:        order.LIMIT,
		Quantity:         qty2,
		Price:            price2,
		NewClientOrderId: clientOrderId2,
	})
	if err != nil {
		return wrap.Errorf("place order 2 (%s %s): %w", side2.String(), chain.symbolAB, err)
	}
	fmt.Printf("[DEBUG] Ордер 2 размещён: orderId=%s\n", order2.OrderID)

	if err := u.waitOrderFilled(ctx, chain.symbolAB, order2.OrderID, "2"); err != nil {
		return err
	}

	// --- Шаг 3: Лимитный ордер SELL B за USDT (количество B получено на шаге 2) ---
	var amountBActual float64
	if chain.usedDirectAB {
		amountBActual = qty2 * chain.priceAB
	} else {
		amountBActual = qty2
	}
	qty3 := roundQty(amountBActual, precisionB)
	price3 := chain.priceBUSDT
	fmt.Printf("[DEBUG] Шаг 3: SELL %s | quantity=%.8f price=%.8f\n", symbolB, qty3, price3)

	clientOrderId3 := fmt.Sprintf("swap_%d_3_%s", runID, best.baseB)

	order3, err := u.repo.NewOrder(model.OrderParams{
		Symbol:           symbolB,
		Side:             order.SELL,
		OrderType:        order.LIMIT,
		Quantity:         qty3,
		Price:            price3,
		NewClientOrderId: clientOrderId3,
	})
	if err != nil {
		return wrap.Errorf("place order 3 (SELL %s): %w", symbolB, err)
	}
	fmt.Printf("[DEBUG] Ордер 3 размещён: orderId=%s\n", order3.OrderID)

	if err := u.waitOrderFilled(ctx, symbolB, order3.OrderID, "3"); err != nil {
		return err
	}

	// --- Итог ---
	accountInfo2, _ := u.repo.GetBalance(ctx)
	usdtBal2, _ := helpers.FindUSDTBalance(accountInfo2.Balances)
	fmt.Printf("\n=== Цепочка выполнена ===\n")
	fmt.Printf("[DEBUG] Баланс USDT после: Free=%.2f Locked=%.2f\n", usdtBal2.Free, usdtBal2.Locked)
	fmt.Printf("Ордера: %s, %s, %s\n", order1.OrderID, order2.OrderID, order3.OrderID)
	return nil
}

func (u *SwapProcess) waitOrderFilled(ctx context.Context, symbol, orderID, stepLabel string) error {
	deadline := time.Now().Add(orderFillWaitTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(orderFillPollInterval)
		q, err := u.repo.GetOrderQuery(symbol, orderID)
		if err != nil {
			return wrap.Errorf("step %s query order: %w", stepLabel, err)
		}
		if q == nil {
			fmt.Printf("[DEBUG] Шаг %s: ордер %s не найден (возможно уже исполнен)\n", stepLabel, orderID)
			return nil
		}
		fmt.Printf("[DEBUG] Шаг %s: статус=%s executedQty=%s\n", stepLabel, q.Status, q.ExecutedQty)
		if q.Status == "FILLED" {
			return nil
		}
		if q.Status == "CANCELED" || q.Status == "REJECTED" || q.Status == "EXPIRED" {
			return wrap.Errorf("step %s: ордер в статусе %s", stepLabel, q.Status)
		}
	}
	return wrap.Errorf("step %s: таймаут ожидания исполнения ордера %s", stepLabel, orderID)
}

func (u *SwapProcess) findSymbolDetail(info *mexc.SymbolInfo, symbol string) *mexc.SymbolDetail {
	for i := range info.Symbols {
		if info.Symbols[i].Symbol == symbol {
			return &info.Symbols[i]
		}
	}
	return nil
}

// calcChainProfit считает цепочку USDT -> A -> B -> USDT (старт с 1 USDT).
func (u *SwapProcess) calcChainProfit(priceMap map[string]float64, coinA, coinB *mexc.SymbolDetail) (swapChainResult, bool) {
	var res swapChainResult
	symbolAUSDT := coinA.BaseAsset + "USDT"
	symbolBUSDT := coinB.BaseAsset + "USDT"
	priceAUSDT, ok := priceMap[symbolAUSDT]
	if !ok || priceAUSDT <= 0 {
		return res, false
	}
	priceBUSDT, ok := priceMap[symbolBUSDT]
	if !ok || priceBUSDT <= 0 {
		return res, false
	}
	res.priceAUSDT = priceAUSDT
	res.priceBUSDT = priceBUSDT
	amountA := 1.0 / priceAUSDT
	if amountA <= 0 {
		return res, false
	}
	res.amountA = amountA
	symbolAB := coinA.BaseAsset + coinB.BaseAsset
	symbolBA := coinB.BaseAsset + coinA.BaseAsset
	if priceAB, ok := priceMap[symbolAB]; ok && priceAB > 0 {
		res.amountB = amountA * priceAB
		res.symbolAB = symbolAB
		res.priceAB = priceAB
		res.usedDirectAB = true
	} else if priceBA, ok := priceMap[symbolBA]; ok && priceBA > 0 {
		res.amountB = amountA / priceBA
		res.symbolAB = symbolBA
		res.priceAB = priceBA
		res.usedDirectAB = false
	} else {
		return res, false
	}
	if res.amountB <= 0 {
		return res, false
	}
	res.amountUSDT = res.amountB * priceBUSDT
	res.profitPercent = (res.amountUSDT - 1.0) * 100.0
	return res, true
}

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
