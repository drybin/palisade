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

// swapLotStep — шаг количества базового актива для округления вниз.
// У MEXC baseSizePrecision часто дробный шаг (например "0.000001"); для части кросс-пар
// приходит целое вроде "1" при baseAssetPrecision=2 (SOLBTC) — это не шаг 1 монета, тогда берём 10^-baseAssetPrecision.
// Если baseAssetPrecision в ответе 0, baseSizePrecision "1" ошибочно даёт шаг 1.0 — поэтому сначала LOT_SIZE.stepSize.
func swapLotStep(sym *mexc.SymbolDetail) (float64, error) {
	if sym == nil {
		return 0, wrap.Errorf("symbol detail is nil")
	}

	for i := range sym.Filters {
		if sym.Filters[i].FilterType != "LOT_SIZE" {
			continue
		}
		raw := strings.TrimSpace(sym.Filters[i].StepSize)
		if raw == "" {
			continue
		}
		step, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		if step > 0 {
			return step, nil
		}
	}

	stepByDecimals := float64(0)
	if sym.BaseAssetPrecision > 0 {
		stepByDecimals = math.Pow(10, -sym.BaseAssetPrecision)
	}

	raw := strings.TrimSpace(sym.BaseSizePrecision)
	if raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, wrap.Errorf("parse baseSizePrecision %q for %s: %w", raw, sym.Symbol, err)
		}
		if parsed > 0 {
			if parsed < 1 {
				return parsed, nil
			}
			if stepByDecimals > 0 {
				return stepByDecimals, nil
			}
			return parsed, nil
		}
	}

	if stepByDecimals > 0 {
		return stepByDecimals, nil
	}
	return 0, wrap.Errorf("no valid lot step for %s (baseSizePrecision=%q baseAssetPrecision=%v)",
		sym.Symbol, sym.BaseSizePrecision, sym.BaseAssetPrecision)
}

// swapRoundQtyDown — количество не больше qty, кратное step (вниз), без math.Floor(qty) для дробей.
func swapRoundQtyDown(qty, step float64) float64 {
	if qty <= 0 || step <= 0 {
		return 0
	}
	return math.Floor(qty/step) * step
}

const (
	minProfitPercent      = 1.0
	amountInUSDT          = 10.0
	orderFillWaitTimeout  = 120 * time.Second
	orderFillPollInterval = 3 * time.Second
	// swapIntermediateBuffer — запас под комиссию и округление между шагами цепочки.
	swapIntermediateBuffer = 0.999
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
	fmt.Println("=== swap_process: поиск выгодной связки и исполнение лимитными ордерами ===")

	// --- 1. Book ticker (best bid/ask) — исполнимые цены для расчёта и лимитов ---
	fmt.Printf("[DEBUG] Загрузка book ticker (bid/ask)...\n")
	bookRows, err := u.repo.GetAllBookTickers(ctx)
	if err != nil {
		return wrap.Errorf("failed to get book tickers: %w", err)
	}
	book := BuildSwapBookMap(bookRows)
	fmt.Printf("[DEBUG] Загружено символов с bid/ask: %d\n\n", len(book))

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
	for symbol := range book {
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
			res, ok := calcSwapChainFromBook(book, &coinA, &coinB)
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
	chain, ok := calcSwapChainFromBook(book, &coinA, &coinB)
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

	stepA, err := swapLotStep(symA)
	if err != nil {
		return wrap.Errorf("%s: %w", symbolA, err)
	}
	stepB, err := swapLotStep(symB)
	if err != nil {
		return wrap.Errorf("%s: %w", symbolB, err)
	}
	stepAB, err := swapLotStep(symAB)
	if err != nil {
		return wrap.Errorf("%s: %w", chain.symbolAB, err)
	}

	qty1 := swapRoundQtyDown(amountInUSDT/chain.priceAUSDT, stepA)
	price1 := chain.priceAUSDT
	if qty1 <= 0 {
		return wrap.Errorf("шаг 1: количество после округления обнулилось (%s step=%g)", symbolA, stepA)
	}

	// Preflight шагов 2–3 по ожидаемому балансу после шага 1 (без math.Floor(qty) при невалидном step).
	qtyAExpected := qty1 * swapIntermediateBuffer
	var qty2pref float64
	if chain.usedDirectAB {
		qty2pref = swapRoundQtyDown(math.Min(qty1, qtyAExpected), stepAB)
	} else {
		planB := qty1 / chain.priceAB
		maxAffordB := qtyAExpected / chain.priceAB
		qty2pref = swapRoundQtyDown(math.Min(planB, maxAffordB), stepAB)
	}
	if qty2pref <= 0 {
		return wrap.Errorf("preflight шаг 2: по паре %s количество обнулилось (step=%g, ожидаемо %s≈%.8f)",
			chain.symbolAB, stepAB, best.baseA, qtyAExpected)
	}
	var amountBpref float64
	if chain.usedDirectAB {
		amountBpref = qty2pref * chain.priceAB
	} else {
		amountBpref = qty2pref
	}
	qty3pref := swapRoundQtyDown(amountBpref, stepB)
	if qty3pref <= 0 {
		return wrap.Errorf("preflight шаг 3: по %s количество обнулилось (step=%g)", symbolB, stepB)
	}

	fmt.Printf("[DEBUG] Шаг 1: BUY %s @ ask | quantity=%.8f price=%.8f (quote≈%.2f USDT)\n", symbolA, qty1, price1, qty1*price1)

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

	acctAfter1, err := u.repo.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("balance after step 1: %w", err)
	}
	balA, err := helpers.FindAssetBalance(acctAfter1.Balances, best.baseA)
	if err != nil {
		return err
	}
	qtyAActual := balA.Free * swapIntermediateBuffer
	fmt.Printf("[DEBUG] Доступно %s для шага 2 (с запасом): %.8f (free=%.8f)\n", best.baseA, qtyAActual, balA.Free)

	// --- Шаг 2: Лимитный ордер на паре A-B по фактическому балансу A (после комиссии шага 1) ---
	// usedDirectAB: пара AB (base=A, quote=B) — продаём A → SELL
	// иначе пара BA (base=B, quote=A) — покупаем B за A → BUY; стоимость в A = qty2*price
	var qty2 float64
	price2 := chain.priceAB
	if chain.usedDirectAB {
		qty2 = swapRoundQtyDown(math.Min(qty1, qtyAActual), stepAB)
	} else {
		planB := qty1 / chain.priceAB
		maxAffordB := qtyAActual / chain.priceAB
		qty2 = swapRoundQtyDown(math.Min(planB, maxAffordB), stepAB)
	}
	if qty2 <= 0 {
		return wrap.Errorf("шаг 2: после учёта баланса %s количество по паре %s обнулилось", best.baseA, chain.symbolAB)
	}

	clientOrderId2 := fmt.Sprintf("swap_%d_2_%s", runID, chain.symbolAB)

	var side2 order.Side
	if chain.usedDirectAB {
		side2 = order.SELL
	} else {
		side2 = order.BUY
	}
	leg2BookSide := "ask"
	if chain.usedDirectAB {
		leg2BookSide = "bid"
	}
	fmt.Printf("[DEBUG] Шаг 2: %s %s @ %s | quantity=%.8f price=%.8f\n", side2.String(), chain.symbolAB, leg2BookSide, qty2, price2)

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
	qty3 := swapRoundQtyDown(amountBActual, stepB)
	price3 := chain.priceBUSDT
	fmt.Printf("[DEBUG] Шаг 3: SELL %s @ bid | quantity=%.8f price=%.8f\n", symbolB, qty3, price3)

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

