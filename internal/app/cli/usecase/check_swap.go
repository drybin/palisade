package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/model/mexc"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type ICheckSwap interface {
    Process(ctx context.Context, quiet bool) error
}

type CheckSwap struct {
    repo      *webapi.MexcWebapi
    stateRepo repo.IStateRepository
}

func NewCheckSwapUsecase(repo *webapi.MexcWebapi, stateRepo repo.IStateRepository) *CheckSwap {
    return &CheckSwap{repo: repo, stateRepo: stateRepo}
}

func (u *CheckSwap) Process(ctx context.Context, quiet bool) error {
    bookRows, err := u.repo.GetAllBookTickers(ctx)
    if err != nil {
        return wrap.Errorf("failed to get book tickers: %w", err)
    }
    book := BuildSwapBookMap(bookRows)
    
    // Тип и расчёт allChains вынесены для использования в обоих режимах
    type chainProfit struct {
        baseA, baseB string
        profit       float64
    }
    allowedHubs := map[string]bool{"BTC": true, "ETH": true, "USDC": true}
    baseAssets := make([]string, 0)
    for symbol := range book {
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
            res, ok := calcSwapChainFromBook(book, &coinA, &coinB)
            if !ok {
                continue
            }
            allChains = append(allChains, chainProfit{baseA, baseB, res.profitPercent})
        }
    }
    sort.Slice(allChains, func(i, j int) bool { return allChains[i].profit > allChains[j].profit })
    
    if quiet {
        n := 0
        for _, c := range allChains {
            if c.profit <= 1 {
                continue
            }
            n++
            fmt.Printf("%d. USDT -> %s -> %s -> USDT  |  %.4f%%\n", n, c.baseA, c.baseB, c.profit)
        }
        return nil
    }
    
    fmt.Printf("Символов с bid/ask в book ticker: %d\n", len(book))
    
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
    symbolB := baseB + "USDT" // BTCUSDT
    
    var coinA, coinB mexc.SymbolDetail
    coinA.Symbol = symbolA
    coinA.BaseAsset = baseA
    coinA.QuoteAsset = "USDT"
    coinB.Symbol = symbolB
    coinB.BaseAsset = baseB
    coinB.QuoteAsset = "USDT"
    
    fmt.Printf("--- Дебаг цепочки USDT -> %s -> %s -> USDT ---\n\n", baseA, baseB)
    
    res, ok := calcSwapChainFromBook(book, &coinA, &coinB)
    if !ok {
        // Выводим, чего не хватает
        fmt.Printf("Цепочка USDT -> %s -> %s -> USDT: не хватает данных.\n", baseA, baseB)
		printBook := func(sym string, needAsk, needBid bool) {
			q, okq := book[sym]
			if !okq {
				fmt.Printf("  Нет в book ticker: %s\n", sym)
				return
			}
			if needAsk && q.Ask <= 0 {
				fmt.Printf("  %s: нет ask (bid=%.8g)\n", sym, q.Bid)
				return
			}
			if needBid && q.Bid <= 0 {
				fmt.Printf("  %s: нет bid (ask=%.8g)\n", sym, q.Ask)
				return
			}
			fmt.Printf("  %s: bid=%.8g ask=%.8g\n", sym, q.Bid, q.Ask)
		}
		printBook(symbolA, true, false)
		printBook(symbolB, false, true)
		symbolAB := baseA + baseB
		symbolBA := baseB + baseA
		printBook(symbolAB, false, true)
		printBook(symbolBA, true, false)
        return nil
    }
    
    fmt.Printf("Цепочка: USDT -> %s -> %s -> USDT\n\n", coinA.BaseAsset, coinB.BaseAsset)
    
    fmt.Println("Шаг 1: Покупаем", coinA.BaseAsset, "за USDT (лимит по best ask)")
    fmt.Printf("  Ask %s: %.8g USDT за 1 %s\n", coinA.Symbol, res.priceAUSDT, coinA.BaseAsset)
    fmt.Printf("  Потратили: 1 USDT\n")
    fmt.Printf("  Получили: 1 / %.8g = %.8g %s\n\n", res.priceAUSDT, res.amountA, coinA.BaseAsset)
    
    fmt.Printf("Шаг 2: Меняем %s на %s\n", coinA.BaseAsset, coinB.BaseAsset)
    if res.usedDirectAB {
        fmt.Printf("  Пара %s: best bid %.8g (SELL %s)\n", res.symbolAB, res.priceAB, coinA.BaseAsset)
    } else {
        fmt.Printf("  Пара %s: best ask %.8g (BUY %s за %s)\n", res.symbolAB, res.priceAB, coinB.BaseAsset, coinA.BaseAsset)
    }
    if res.usedDirectAB {
        fmt.Printf("  Отдаём: %.8g %s\n", res.amountA, coinA.BaseAsset)
        fmt.Printf("  Получаем: %.8g * %.8g = %.8g %s\n\n", res.amountA, res.priceAB, res.amountB, coinB.BaseAsset)
    } else {
        fmt.Printf("  Отдаём: %.8g %s\n", res.amountA, coinA.BaseAsset)
        fmt.Printf("  Получаем: %.8g / %.8g = %.8g %s\n\n", res.amountA, res.priceAB, res.amountB, coinB.BaseAsset)
    }
    
    fmt.Printf("Шаг 3: Продаём %s за USDT (лимит по best bid)\n", coinB.BaseAsset)
    fmt.Printf("  Bid %s: %.8g USDT за 1 %s\n", coinB.Symbol, res.priceBUSDT, coinB.BaseAsset)
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
    
    // Все цепочки с прибылью (allChains уже посчитаны выше)
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
        fmt.Println("  Нет цепочек с полными данными (нет пар A-B в book ticker).")
    }
    
    return nil
}

