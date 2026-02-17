package usecase

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckSwap interface {
	Process(ctx context.Context) error
}

type CheckSwap struct {
	repo      *webapi.MexcWebapi
	stateRepo repo.IStateRepository
}

func NewCheckSwapUsecase(repo *webapi.MexcWebapi, stateRepo repo.IStateRepository) *CheckSwap {
	return &CheckSwap{repo: repo, stateRepo: stateRepo}
}

func (u *CheckSwap) Process(ctx context.Context) error {
	tickers, err := u.repo.GetAllTickerPrices(ctx)
	if err != nil {
		return wrap.Errorf("failed to get ticker prices: %w", err)
	}

	// Карта символов пар -> цена (float64)
	priceMap := make(map[string]float64, len(*tickers))
	for _, t := range *tickers {
		p, err := strconv.ParseFloat(t.Price, 64)
		if err != nil || p <= 0 {
			continue
		}
		priceMap[t.Symbol] = p
	}
	fmt.Printf("Тикеров с ценой: %d\n", len(priceMap))

	spotTradingAllowed := true
	allCoins, err := u.stateRepo.GetCoins(ctx, repo.GetCoinsParams{
		IsSpotTradingAllowed: &spotTradingAllowed,
		Limit:                10000,
		Offset:               0,
	})
	if err != nil {
		return wrap.Errorf("failed to get coins: %w", err)
	}

	// Только пары с quoteAsset = USDT
	var coins []mexc.SymbolDetail
	for _, c := range allCoins {
		if c.QuoteAsset == "USDT" {
			coins = append(coins, c)
		}
	}
	fmt.Printf("Монет с QuoteAsset = USDT: %d\n\n", len(coins))

	// Вывод монет из БД, которых нет в тикерах — закомментирован
	// var missingInTickers []mexc.SymbolDetail
	// for _, c := range coins {
	// 	if _, ok := priceMap[c.Symbol]; !ok {
	// 		missingInTickers = append(missingInTickers, c)
	// 	}
	// }
	// fmt.Printf("Монет из БД, которых нет в тикерах: %d\n", len(missingInTickers))
	// for i, c := range missingInTickers {
	// 	fmt.Printf("  %d. %s (Base: %s, Quote: %s)\n", i+1, c.Symbol, c.BaseAsset, c.QuoteAsset)
	// }
	// fmt.Println()

	// Расчёт для фиксированной цепочки USDT -> ATOM -> BTC -> USDT с подробным дебагом
	const baseA, baseB = "ATOM", "BTC"
	symbolA := baseA + "USDT" // ATOMUSDT
	symbolB := baseB + "USDT"  // BTCUSDT

	var coinA, coinB mexc.SymbolDetail
	coinA.Symbol = symbolA
	coinA.BaseAsset = baseA
	coinA.QuoteAsset = "USDT"
	coinB.Symbol = symbolB
	coinB.BaseAsset = baseB
	coinB.QuoteAsset = "USDT"

	fmt.Printf("--- Дебаг цепочки USDT -> %s -> %s -> USDT ---\n\n", baseA, baseB)

	res, ok := u.calcChainProfit(priceMap, &coinA, &coinB)
	if !ok {
		// Выводим, чего не хватает
		fmt.Printf("Цепочка USDT -> %s -> %s -> USDT: не хватает данных.\n", baseA, baseB)
		if _, ok1 := priceMap[symbolA]; !ok1 {
			fmt.Printf("  Нет в тикерах: %s\n", symbolA)
		} else {
			fmt.Printf("  %s: есть, цена = %.8g\n", symbolA, priceMap[symbolA])
		}
		if _, ok2 := priceMap[symbolB]; !ok2 {
			fmt.Printf("  Нет в тикерах: %s\n", symbolB)
		} else {
			fmt.Printf("  %s: есть, цена = %.8g\n", symbolB, priceMap[symbolB])
		}
		symbolAB := baseA + baseB
		symbolBA := baseB + baseA
		if _, ok3 := priceMap[symbolAB]; !ok3 {
			fmt.Printf("  Нет в тикерах: %s\n", symbolAB)
		} else {
			fmt.Printf("  %s: есть, цена = %.8g\n", symbolAB, priceMap[symbolAB])
		}
		if _, ok4 := priceMap[symbolBA]; !ok4 {
			fmt.Printf("  Нет в тикерах: %s\n", symbolBA)
		} else {
			fmt.Printf("  %s: есть, цена = %.8g\n", symbolBA, priceMap[symbolBA])
		}
		return nil
	}

	fmt.Printf("Цепочка: USDT -> %s -> %s -> USDT\n\n", coinA.BaseAsset, coinB.BaseAsset)

	fmt.Println("Шаг 1: Покупаем", coinA.BaseAsset, "за USDT")
	fmt.Printf("  Цена %s: %.8g USDT за 1 %s\n", coinA.Symbol, res.priceAUSDT, coinA.BaseAsset)
	fmt.Printf("  Потратили: 1 USDT\n")
	fmt.Printf("  Получили: 1 / %.8g = %.8g %s\n\n", res.priceAUSDT, res.amountA, coinA.BaseAsset)

	fmt.Printf("Шаг 2: Меняем %s на %s\n", coinA.BaseAsset, coinB.BaseAsset)
	fmt.Printf("  Пара %s: %.8g (за 1 %s получаем столько %s)\n", res.symbolAB, res.priceAB, coinA.BaseAsset, coinB.BaseAsset)
	if res.usedDirectAB {
		fmt.Printf("  Отдаём: %.8g %s\n", res.amountA, coinA.BaseAsset)
		fmt.Printf("  Получаем: %.8g * %.8g = %.8g %s\n\n", res.amountA, res.priceAB, res.amountB, coinB.BaseAsset)
	} else {
		fmt.Printf("  Отдаём: %.8g %s\n", res.amountA, coinA.BaseAsset)
		fmt.Printf("  Получаем: %.8g / %.8g = %.8g %s\n\n", res.amountA, res.priceAB, res.amountB, coinB.BaseAsset)
	}

	fmt.Printf("Шаг 3: Продаём %s за USDT\n", coinB.BaseAsset)
	fmt.Printf("  Цена %s: %.8g USDT за 1 %s\n", coinB.Symbol, res.priceBUSDT, coinB.BaseAsset)
	fmt.Printf("  Отдаём: %.8g %s\n", res.amountB, coinB.BaseAsset)
	fmt.Printf("  Получаем: %.8g * %.8g = %.8g USDT\n\n", res.amountB, res.priceBUSDT, res.amountUSDT)

	fmt.Println("Итог:")
	fmt.Printf("  Вложили: 1 USDT\n")
	fmt.Printf("  Получили: %.8g USDT\n", res.amountUSDT)
	if res.profitPercent >= 0 {
		fmt.Printf("  Плюс: %.4f%%\n", res.profitPercent)
	} else {
		fmt.Printf("  Минус: %.4f%%\n", -res.profitPercent)
	}

	// Только связки через BTC, ETH или USDC. Базы берём из тикеров (как для ATOM), не из БД.
	allowedHubs := map[string]bool{"BTC": true, "ETH": true, "USDC": true}
	type chainProfit struct {
		baseA, baseB string
		profit       float64
	}
	baseAssets := make([]string, 0)
	for symbol := range priceMap {
		if strings.HasSuffix(symbol, "USDT") && len(symbol) > 4 {
			base := symbol[:len(symbol)-4]
			baseAssets = append(baseAssets, base)
		}
	}
	sort.Strings(baseAssets)
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
			coinA := mexc.SymbolDetail{BaseAsset: baseA, Symbol: baseA + "USDT", QuoteAsset: "USDT"}
			coinB := mexc.SymbolDetail{BaseAsset: baseB, Symbol: baseB + "USDT", QuoteAsset: "USDT"}
			res, ok := u.calcChainProfit(priceMap, &coinA, &coinB)
			if !ok {
				continue
			}
			allChains = append(allChains, chainProfit{baseA, baseB, res.profitPercent})
		}
	}
	sort.Slice(allChains, func(i, j int) bool { return allChains[i].profit > allChains[j].profit })

	// Все цепочки с прибылью (те же данные, что и топ-5)
	fmt.Printf("\n--- Все цепочки с прибылью (USDT -> A -> B -> USDT, через BTC/ETH/USDC) ---\n")
	var profitable int
	for _, c := range allChains {
		if c.profit <= 0 {
			continue
		}
		profitable++
		fmt.Printf("  %d. USDT -> %s -> %s -> USDT  |  прибыль: %.4f%%\n",
			profitable, c.baseA, c.baseB, c.profit)
	}
	if profitable == 0 {
		fmt.Println("  Нет цепочек с прибылью.")
	} else {
		fmt.Printf("\nВсего цепочек с прибылью: %d\n", profitable)
	}

	fmt.Printf("\n--- Топ 5 связок с минимальным минусом (только через BTC/ETH/USDC) ---\n")
	top := 5
	if len(allChains) < top {
		top = len(allChains)
	}
	for i := 0; i < top; i++ {
		c := allChains[i]
		fmt.Printf("  %d. USDT -> %s -> %s -> USDT  |  %.4f%%\n", i+1, c.baseA, c.baseB, c.profit)
	}
	if top == 0 {
		fmt.Println("  Нет цепочек с полными данными (нет пар A-B в тикерах).")
	}

	return nil
}

// calcChainProfit считает цепочку USDT -> A -> B -> USDT (старт с 1 USDT). Без дебаг-строк.
// Для пары с USDT используем BaseAsset+"USDT" (как в тикерах биржи), а не coin.Symbol из БД.
func (u *CheckSwap) calcChainProfit(priceMap map[string]float64, coinA, coinB *mexc.SymbolDetail) (chainResult, bool) {
	var res chainResult
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

// chainResult — результат расчёта цепочки USDT -> A -> B -> USDT.
type chainResult struct {
	priceAUSDT    float64
	priceBUSDT    float64
	symbolAB      string  // какая пара использована (AB или BA)
	priceAB       float64
	amountA       float64
	amountB       float64
	amountUSDT    float64
	profitPercent float64
	usedDirectAB  bool    // true: amountB = amountA * price; false: amountB = amountA / price
}

// calcChainProfitWithDebug считает цепочку и возвращает дебаг-строки по каждому шагу проверки.
// Для пары с USDT используем BaseAsset+"USDT" (как в тикерах биржи).
func (u *CheckSwap) calcChainProfitWithDebug(priceMap map[string]float64, coinA, coinB *mexc.SymbolDetail) (chainResult, bool, []string) {
	var res chainResult
	var lines []string
	symbolAUSDT := coinA.BaseAsset + "USDT"
	symbolBUSDT := coinB.BaseAsset + "USDT"

	priceAUSDT, okA := priceMap[symbolAUSDT]
	if !okA || priceAUSDT <= 0 {
		lines = append(lines, fmt.Sprintf("    %s: нет в тикерах", symbolAUSDT))
		return res, false, lines
	}
	lines = append(lines, fmt.Sprintf("    %s: ok = %.8g", symbolAUSDT, priceAUSDT))
	res.priceAUSDT = priceAUSDT

	priceBUSDT, okB := priceMap[symbolBUSDT]
	if !okB || priceBUSDT <= 0 {
		lines = append(lines, fmt.Sprintf("    %s: нет в тикерах", symbolBUSDT))
		return res, false, lines
	}
	lines = append(lines, fmt.Sprintf("    %s: ok = %.8g", symbolBUSDT, priceBUSDT))
	res.priceBUSDT = priceBUSDT

	amountA := 1.0 / priceAUSDT
	if amountA <= 0 {
		lines = append(lines, "    расчёт amountA: ошибка (деление на ноль)")
		return res, false, lines
	}
	res.amountA = amountA

	symbolAB := coinA.BaseAsset + coinB.BaseAsset
	symbolBA := coinB.BaseAsset + coinA.BaseAsset
	var amountB float64
	if priceAB, ok := priceMap[symbolAB]; ok && priceAB > 0 {
		amountB = amountA * priceAB
		res.symbolAB = symbolAB
		res.priceAB = priceAB
		res.usedDirectAB = true
		lines = append(lines, fmt.Sprintf("    %s: ok = %.8g (используем для A->B)", symbolAB, priceAB))
	} else if priceBA, ok := priceMap[symbolBA]; ok && priceBA > 0 {
		amountB = amountA / priceBA
		res.symbolAB = symbolBA
		res.priceAB = priceBA
		res.usedDirectAB = false
		lines = append(lines, fmt.Sprintf("    %s: ok = %.8g (используем для A->B)", symbolBA, priceBA))
	} else {
		lines = append(lines, fmt.Sprintf("    пара A-B: нет (проверены %s, %s)", symbolAB, symbolBA))
		return res, false, lines
	}
	if amountB <= 0 {
		lines = append(lines, "    расчёт amountB: ошибка")
		return res, false, lines
	}
	res.amountB = amountB

	amountUSDT := amountB * priceBUSDT
	res.amountUSDT = amountUSDT
	res.profitPercent = (amountUSDT - 1.0) * 100.0
	lines = append(lines, fmt.Sprintf("    цены для расчёта: %s=%.8g  %s=%.8g  %s=%.8g", coinA.Symbol, res.priceAUSDT, res.symbolAB, res.priceAB, coinB.Symbol, res.priceBUSDT))
	return res, true, lines
}
