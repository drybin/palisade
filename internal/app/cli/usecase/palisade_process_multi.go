package usecase

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/enum/order"
	"github.com/drybin/palisade/internal/domain/helpers"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/wrap"
)

type IPalisadeProcessMulti interface {
	Process(ctx context.Context) error
}

type PalisadeProcessMulti struct {
	repo                  *webapi.MexcWebapi
	repoV2                *webapi.MexcV2Webapi
	telegramApi           *webapi.TelegramWebapi
	traidingPairsService  *service.TradingPair
	palisadeLevelsService *service.PalisadeLevels
	buyService            *service.ByuService
	checkerService        *service.PalisadeCheckerService
	stateRepo             repo.IStateRepository
}

func NewPalisadeProcessMultiUsecase(
	repo *webapi.MexcWebapi,
	repoV2 *webapi.MexcV2Webapi,
	telegramApi *webapi.TelegramWebapi,
	traidingPairsService *service.TradingPair,
	palisadeLevelsService *service.PalisadeLevels,
	buyService *service.ByuService,
	checkerService *service.PalisadeCheckerService,
	stateRepo repo.IStateRepository,
) *PalisadeProcessMulti {
	return &PalisadeProcessMulti{
		repo:                  repo,
		repoV2:                repoV2,
		telegramApi:           telegramApi,
		traidingPairsService:  traidingPairsService,
		palisadeLevelsService: palisadeLevelsService,
		buyService:            buyService,
		checkerService:        checkerService,
		stateRepo:             stateRepo,
	}
}

func (u *PalisadeProcessMulti) Process(ctx context.Context) error {
	fmt.Println("palisade process multi")

	maxOrderCount := 5
	orderAmount := 2
	accountInfo, err := u.repo.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("failed to get balance: %w", err)
	}

	ordersFromTradeLog, err := u.stateRepo.GetOpenOrders(ctx)
	if err != nil {
		return wrap.Errorf("failed to get orders from trade_log: %w", err)
	}

	cntOrdesFromTradeLog := len(ordersFromTradeLog)
	fmt.Printf("\n=== Найдено ордеров в trade_log %d ===\n", cntOrdesFromTradeLog)
	if cntOrdesFromTradeLog >= maxOrderCount {
		fmt.Printf("Найдено %d или больше открытых ордеров (макс. %d), прекращаем работу\n", cntOrdesFromTradeLog, maxOrderCount)
		return nil
	}

	usdtBalance, err := helpers.FindUSDTBalance(accountInfo.Balances)
	if err != nil {
		return wrap.Errorf("failed to find USDT balance: %w", err)
	}
	fmt.Printf("USDT Balance: %f\n", usdtBalance.Free)

	// Рассчитываем сколько еще ордеров можно разместить
	remainingOrders := maxOrderCount - cntOrdesFromTradeLog
	requiredBalance := float64(remainingOrders) * float64(orderAmount)

	fmt.Printf("Можно разместить еще ордеров: %d\n", remainingOrders)
	fmt.Printf("Требуется баланс для %d ордеров: %.2f USDT (по %.2f USDT каждый)\n", remainingOrders, requiredBalance, float64(orderAmount))

	if usdtBalance.Free < requiredBalance {
		fmt.Printf("❌ Недостаточно баланса: доступно %.2f USDT, требуется %.2f USDT\n", usdtBalance.Free, requiredBalance)
		return nil
	}

	fmt.Printf("✅ Баланса достаточно для размещения %d ордеров\n", remainingOrders)

	coins, err := u.stateRepo.GetCoinsToProcess(ctx, 50, 0)
	if err != nil {
		return wrap.Errorf("failed to get coins to process TPTU: %w", err)
	}
	if len(coins) == 0 {
		return wrap.Errorf("no coins to process")
	}

	// Случайно выбираем монеты в количестве remainingOrders
	fmt.Printf("\n=== Выбор монет для обработки ===\n")
	fmt.Printf("Всего монет доступно: %d\n", len(coins))
	fmt.Printf("Нужно выбрать: %d\n", remainingOrders)

	// Если монет меньше чем нужно, берем все доступные
	selectedCount := remainingOrders
	if len(coins) < remainingOrders {
		selectedCount = len(coins)
		fmt.Printf("⚠️ Доступно только %d монет, выбираем все\n", selectedCount)
	}

	// Перемешиваем монеты случайным образом
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(coins), func(i, j int) {
		coins[i], coins[j] = coins[j], coins[i]
	})

	// Берем первые selectedCount монет после перемешивания
	selectedCoins := coins[:selectedCount]

	fmt.Printf("✅ Выбрано %d монет для обработки:\n", len(selectedCoins))
	for i, coin := range selectedCoins {
		fmt.Printf("  %d. %s\n", i+1, coin.Symbol)
	}
	fmt.Println()

	// Структура для хранения данных об успешных ордерах
	type OrderInfo struct {
		Symbol      string
		OrderID     string
		Support     float64
		Resistance  float64
		Quantity    float64
		TotalAmount float64
	}
	var successfulOrders []OrderInfo

	// Обрабатываем каждую выбранную монету
	successCount := 0
	for coinIndex, coin := range selectedCoins {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Обработка монеты %d/%d: %s\n", coinIndex+1, len(selectedCoins), coin.Symbol)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		// Обновляем данные монеты через сервис для получения актуальных параметров
		fmt.Printf("Обновление данных для монеты: %s\n", coin.Symbol)
		checkResult, err := u.checkerService.CheckAndUpdateCoin(ctx, service.CheckCoinParams{
			Symbol:                coin.Symbol,
			BaseAsset:             coin.BaseAsset,
			QuoteAsset:            coin.QuoteAsset,
			LastCheck:             time.Time{}, // Не проверяем время последней проверки
			MinTimeSinceLastCheck: 0,           // Всегда обновляем
			MaxVolatilityPercent:  5.0,
			Percentile:            90.0,
			Debug:                 false, // Не выводим детальный лог
		})

		if err != nil {
			fmt.Printf("❌ Ошибка обновления монеты %s: %v\n", coin.Symbol, err)
			continue
		}

		if checkResult.Skipped {
			fmt.Printf("⚠️  Монета %s пропущена: %s\n", coin.Symbol, checkResult.SkipReason)
			continue
		}

		// Если монета не во флете после обновления, переходим к следующей
		if !checkResult.IsFlat {
			fmt.Printf("❌ Монета %s больше не во флете после обновления\n", coin.Symbol)
			continue
		}

		// Обновляем локальные данные из результата проверки
		coin.Support = checkResult.FlatAnalysis.Support
		coin.Resistance = checkResult.FlatAnalysis.Resistance
		coin.RangeValue = checkResult.FlatAnalysis.Range
		coin.RangePercent = checkResult.FlatAnalysis.RangePercent
		coin.AvgPrice = checkResult.FlatAnalysis.AvgPrice
		coin.Volatility = checkResult.FlatAnalysis.Volatility
		coin.MaxDrawdown = checkResult.FlatAnalysis.MaxDrawdown
		coin.MaxRise = checkResult.FlatAnalysis.MaxRise
		coin.IsPalisade = checkResult.IsFlat

		fmt.Printf("✅ Данные монеты обновлены\n")

		fmt.Printf("\n=== Анализ для %s ===\n", coin.Symbol)
		fmt.Printf("Во флете: %v\n", coin.IsPalisade)
		supportPlus01Percent := coin.Support * 1.001
		fmt.Printf("Нижняя граница (Support): %.8f  (+0.1%% %.8f)\n", coin.Support, supportPlus01Percent)
		//fmt.Printf("Нижняя граница +0.1%%: %.8f\n", supportPlus01Percent)
		resistanceMinus01Percent := coin.Resistance * 0.999
		//fmt.Printf("Верхняя граница -0.1%%: %.8f\n", resistanceMinus01Percent)
		fmt.Printf("Верхняя граница (Resistance): %.8f (-0.1%% %.8f)\n", coin.Resistance, resistanceMinus01Percent)
		fmt.Printf("Диапазон: %.8f\n", coin.RangeValue)
		fmt.Printf("Диапазон в процентах: %.2f%%\n", coin.RangePercent)
		fmt.Printf("Средняя цена: %.8f\n", coin.AvgPrice)
		fmt.Printf("Волатильность: %.2f%%\n", coin.Volatility)
		fmt.Printf("Максимальная просадка: %.2f%%\n", coin.MaxDrawdown)
		fmt.Printf("Максимальный рост: %.2f%%\n", coin.MaxRise)
		fmt.Printf("================================\n\n")

		currentAvgPrice, err := u.repo.GetAvgPrice(ctx, coin.Symbol)
		if err != nil {
			fmt.Printf("❌ Ошибка получения средней цены для %s: %v\n", coin.Symbol, err)
			continue
		}

		currentPrice := currentAvgPrice.Price
		fmt.Printf("Текущая средняя цена: %.8f\n", currentPrice)
		fmt.Printf("Диапазон: %.8f - %.8f\n", supportPlus01Percent, resistanceMinus01Percent)

		// Проверяем, что текущая цена находится внутри диапазона
		//if currentPrice >= supportPlus01Percent && currentPrice <= resistanceMinus01Percent {
		if currentPrice >= coin.Support && currentPrice <= coin.Resistance {
			fmt.Printf("✓ Цена находится в диапазоне\n")
		} else {
			fmt.Printf("✗ Цена ВНЕ диапазона\n")
			if currentPrice < supportPlus01Percent {
				fmt.Printf("  Цена ниже нижней границы на %.8f (%.2f%%)\n",
					supportPlus01Percent-currentPrice,
					((supportPlus01Percent-currentPrice)/supportPlus01Percent)*100)
			} else {
				fmt.Printf("  Цена выше верхней границы на %.8f (%.2f%%)\n",
					currentPrice-resistanceMinus01Percent,
					((currentPrice-resistanceMinus01Percent)/resistanceMinus01Percent)*100)
			}
			continue
		}

		quantity := 2.0 / coin.Support

		// Округлить количество согласно baseSizePrecision
		baseSizePrecision, err := strconv.ParseFloat(coin.BaseSizePrecision, 64)
		if err != nil {
			fmt.Printf("❌ Ошибка парсинга baseSizePrecision для %s: %v\n", coin.Symbol, err)
			continue
		}

		if baseSizePrecision == 0 {
			// Если baseSizePrecision равно 0, округлить до ближайшего целого в меньшую сторону
			quantity = math.Floor(quantity)
			fmt.Printf("📏 Округление количества до целого: %.8f → %.8f (baseSizePrecision: %.8f)\n",
				2.0/coin.Support, quantity, baseSizePrecision)
		} else {
			// Округлить количество до ближайшего кратного baseSizePrecision
			quantity = math.Floor(quantity/baseSizePrecision) * baseSizePrecision
			fmt.Printf("📏 Округление количества: %.8f → %.8f (baseSizePrecision: %.8f)\n",
				2.0/coin.Support, quantity, baseSizePrecision)
		}

		if quantity <= 0 {
			fmt.Printf("❌ Округленное количество %f недопустимо для ордера %s\n", quantity, coin.Symbol)
			continue
		}

		nextOrderId, err := u.stateRepo.GetNextTradeId(ctx)
		if err != nil {
			fmt.Printf("❌ Ошибка получения ID ордера для %s: %v\n", coin.Symbol, err)
			continue
		}
		clientOrderId := fmt.Sprintf("Prod_order_%d", nextOrderId)

		fmt.Printf("\n--- Размещаем ордер %s ---\n", coin.Symbol)
		fmt.Printf("Цена: %.8f\n", coin.Support)
		fmt.Printf("Количество: %.8f\n", quantity)

		placeOrderResult, err := u.repo.NewOrder(
			model.OrderParams{
				Symbol:           coin.Symbol,
				Side:             order.BUY,
				OrderType:        order.LIMIT,
				Quantity:         quantity,
				QuoteOrderQty:    quantity,
				Price:            coin.Support,
				NewClientOrderId: clientOrderId,
			},
		)

		if err != nil {
			fmt.Printf("❌ Ошибка размещения ордера для %s: %v\n", coin.Symbol, err)
			continue
		}

		fmt.Printf("\n✅ Ордер размещен id %s\n", placeOrderResult.OrderID)

		_, err = u.stateRepo.SaveTradeLog(
			ctx,
			repo.SaveTradeLogParams{
				OpenDate:    time.Now(),
				OpenBalance: usdtBalance.Free,
				Symbol:      coin.Symbol,
				BuyPrice:    coin.Support,
				Amount:      quantity,
				OrderId:     placeOrderResult.OrderID,
				UpLevel:     coin.Resistance,
				DownLevel:   coin.Support,
			},
		)

		if err != nil {
			fmt.Printf("❌ Ошибка сохранения ордера в БД для %s: %v\n", coin.Symbol, err)
			continue
		}

		// Сохраняем информацию об успешном ордере
		successfulOrders = append(successfulOrders, OrderInfo{
			Symbol:      coin.Symbol,
			OrderID:     placeOrderResult.OrderID,
			Support:     coin.Support,
			Resistance:  coin.Resistance,
			Quantity:    quantity,
			TotalAmount: coin.Support * quantity,
		})

		successCount++
		fmt.Printf("\n✅ Монета %s успешно обработана! (%d/%d)\n", coin.Symbol, successCount, len(selectedCoins))

		// Обновляем баланс после успешного размещения ордера
		accountInfo, err = u.repo.GetBalance(ctx)
		if err != nil {
			fmt.Printf("⚠️  Не удалось обновить баланс: %v\n", err)
		} else {
			usdtBalance, err = helpers.FindUSDTBalance(accountInfo.Balances)
			if err != nil {
				fmt.Printf("⚠️  Не удалось найти USDT баланс: %v\n", err)
			} else {
				fmt.Printf("📊 Обновленный баланс: %.2f USDT (свободно) / %.2f USDT (заблокировано)\n",
					usdtBalance.Free, usdtBalance.Locked)
			}
		}
	}

	// Итоговая статистика
	fmt.Printf("\n═══════════════════════════════════════\n")
	fmt.Printf("📊 ИТОГИ ОБРАБОТКИ\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("Обработано монет: %d/%d\n", len(selectedCoins), len(selectedCoins))
	fmt.Printf("Успешно размещено ордеров: %d\n", successCount)
	fmt.Printf("Пропущено: %d\n", len(selectedCoins)-successCount)
	fmt.Printf("═══════════════════════════════════════\n")

	// Отправляем общий отчет в Telegram, если есть успешные ордера
	if len(successfulOrders) > 0 {
		// Получаем финальный баланс
		finalAccountInfo, err := u.repo.GetBalance(ctx)
		var finalBalance, finalFree, finalLocked float64
		if err == nil {
			finalUsdtBalance, err := helpers.FindUSDTBalance(finalAccountInfo.Balances)
			if err == nil {
				finalBalance = finalUsdtBalance.Free + finalUsdtBalance.Locked
				finalFree = finalUsdtBalance.Free
				finalLocked = finalUsdtBalance.Locked
			}
		}

		// Формируем общий отчет
		message := "<b>📊 ОТЧЕТ: Размещено несколько ордеров</b>\n\n"
		message += fmt.Sprintf("<b>Всего ордеров:</b> %d\n\n", len(successfulOrders))

		totalAmount := 0.0
		for i, order := range successfulOrders {
			message += fmt.Sprintf("<b>%d. %s</b>\n", i+1, order.Symbol)
			message += fmt.Sprintf("   ID: <code>%s</code>\n", order.OrderID)
			message += fmt.Sprintf("   Support: %.8f\n", order.Support)
			message += fmt.Sprintf("   Resistance: %.8f\n", order.Resistance)
			message += fmt.Sprintf("   Количество: %.8f\n", order.Quantity)
			message += fmt.Sprintf("   Сумма: %.2f USDT\n\n", order.TotalAmount)
			totalAmount += order.TotalAmount
		}

		message += fmt.Sprintf("<b>Итого потрачено:</b> %.2f USDT\n\n", totalAmount)
		message += fmt.Sprintf("<b>Баланс на бирже:</b> %.2f USDT\n", finalBalance)
		message += fmt.Sprintf("  Свободно: %.2f USDT\n", finalFree)
		message += fmt.Sprintf("  Заблокировано: %.2f USDT", finalLocked)

		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Ошибка при отправке отчета в Telegram: %v\n", err)
		} else {
			fmt.Printf("✅ Отчет отправлен в Telegram\n")
		}
	}

	return nil
}
