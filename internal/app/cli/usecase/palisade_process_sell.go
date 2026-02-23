package usecase

import (
	"context"
	"fmt"
	"math"
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

type IPalisadeProcessSell interface {
	Process(ctx context.Context) error
}

type PalisadeProcessSell struct {
	repo        *webapi.MexcWebapi
	stateRepo   repo.IStateRepository
	telegramApi *webapi.TelegramWebapi
}

func NewPalisadeProcessSellUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
	telegramApi *webapi.TelegramWebapi,
) *PalisadeProcessSell {
	return &PalisadeProcessSell{
		repo:        repo,
		stateRepo:   stateRepo,
		telegramApi: telegramApi,
	}
}

func (u *PalisadeProcessSell) Process(ctx context.Context) error {
	fmt.Println("=== Palisade Process Sell ===")

	// Получаем открытые ордера из базы данных
	dbOrders, err := u.stateRepo.GetOpenOrders(ctx)
	if err != nil {
		return wrap.Errorf("failed to get open orders from database: %w", err)
	}

	if len(dbOrders) == 0 {
		fmt.Println("Нет открытых ордеров в базе данных")
		return nil
	}

	// if len(dbOrders) > 1 {
	// 	fmt.Printf("Найдено открытых ордеров в базе данных: %d\n", len(dbOrders))
	// 	fmt.Println("Прекращаем работу, так как открытых ордеров больше 1")
	// 	return nil
	// }

	// Проверяем статус только если ордер один
	dbOrder := dbOrders[0]
	fmt.Printf("\nНайден открытый ордер в базе данных\n\n")

	fmt.Printf("--- Ордер ---\n")
	fmt.Printf("ID в БД: %d\n", dbOrder.ID)
	fmt.Printf("Символ: %s\n", dbOrder.Symbol)
	fmt.Printf("OrderId (биржи): %s\n", dbOrder.OrderId)
	fmt.Printf("Цена покупки: %.8f\n", dbOrder.BuyPrice)
	fmt.Printf("Количество: %.8f\n", dbOrder.Amount)
	fmt.Printf("Дата открытия: %s\n", dbOrder.OpenDate.Format("2006-01-02 15:04:05"))

	// Получаем открытые ордера с биржи для этого символа
	//exchangeOrders, err := u.repo.GetOpenOrders(ctx, model.OrderParams{
	//	Symbol: dbOrder.Symbol,
	//})
	//if err != nil {
	//	return wrap.Errorf("ошибка при получении ордеров с биржи для %s: %w", dbOrder.Symbol, err)
	//}

	// Определяем какой ордер проверять: если есть ордер на продажу, проверяем его, иначе - ордер на покупку
	orderID := dbOrders[0].OrderId
	if dbOrders[0].OrderId_sell != "" {
		orderID = dbOrders[0].OrderId_sell
		fmt.Printf("Проверяем статус ордера на продажу (SELL): %s\n", orderID)
	} else {
		fmt.Printf("Проверяем статус ордера на покупку (BUY): %s\n", orderID)
	}

	queryResult, err := u.repo.GetOrderQuery(dbOrders[0].Symbol, orderID)
	if err != nil {
		return wrap.Errorf("ошибка при получении ордера с биржи для %s: %w", dbOrder.Symbol, err)
	}
	//fmt.Printf("%v\n", queryResult.Status)
	//os.Exit(1)

	if queryResult == nil {
		fmt.Printf("⚠️  Статус: Ордер НЕ найден на бирже (возможно, уже исполнен или отменен)\n")
		// Обновляем cancel_date в базе данных (в часовом поясе GMT+7)
		cancelTime := helpers.NowGMT7()
		fmt.Printf("   Сохраняем cancel_date: %s (часовой пояс: %s)\n", cancelTime.Format("2006-01-02 15:04:05 MST"), cancelTime.Location().String())
		err = u.stateRepo.UpdateCancelDateTradeLog(ctx, dbOrder.ID, cancelTime)
		if err != nil {
			return wrap.Errorf("failed to update cancel date for trade log id %d: %w", dbOrder.ID, err)
		}
		fmt.Printf("✅ Обновлен cancel_date в базе данных\n")

		// Отправляем сообщение в Telegram
		message := fmt.Sprintf(
			"<b>⚠️ Ордер не найден на бирже</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер не найден среди открытых ордеров на бирже (возможно, уже исполнен или отменен)\n"+
				"<b>Действие:</b> Ордер помечен как отмененный в базе данных",
			dbOrder.Symbol,
			dbOrder.OrderId,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			cancelTime.Format("2006-01-02 15:04:05 MST"),
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
		return nil
	}

	//Обрабатываем ордер Sell
	if queryResult.Side == order.SELL.String() {
		if queryResult.Status != "NEW" {
			fmt.Printf("\n✅ Ордер на продажу завершен\n")
			fmt.Printf("Статус: %s\n", queryResult.Status)
			fmt.Printf("Symbol: %s\n", queryResult.Symbol)
			fmt.Printf("OrderID: %s\n", queryResult.OrderID)
			fmt.Printf("Тип ордера: %s\n", queryResult.Type)
			fmt.Printf("Количество исполнено: %s\n", queryResult.ExecutedQty)
			fmt.Printf("Итоговая сумма (USDT): %s\n", queryResult.CummulativeQuoteQty)

			// Обновляем close_date в базе данных
			closeTime := helpers.NowGMT7()

			// Вычисляем финальный баланс и цену продажи
			executedQty, _ := strconv.ParseFloat(queryResult.ExecutedQty, 64)
			// Используем CummulativeQuoteQty - это итоговая сумма в USDT
			closeBalance, _ := strconv.ParseFloat(queryResult.CummulativeQuoteQty, 64)
			// Рассчитываем среднюю цену продажи
			sellPrice := closeBalance / executedQty

			fmt.Printf("ExecutedQty: %s\n", queryResult.ExecutedQty)
			fmt.Printf("CummulativeQuoteQty: %s\n", queryResult.CummulativeQuoteQty)
			fmt.Printf("Средняя цена продажи: %.8f\n", sellPrice)
			fmt.Printf("Итоговая сумма: %.2f USDT\n", closeBalance)

			err = u.stateRepo.UpdateSuccesTradeLog(ctx, dbOrder.ID, closeTime, closeBalance, sellPrice)
			if err != nil {
				return wrap.Errorf("failed to update success trade log for id %d: %w", dbOrder.ID, err)
			}
			fmt.Printf("✅ Обновлен close_date в базе данных\n")

			// Отправляем сообщение в Telegram
			profit := closeBalance - dbOrder.OpenBalance
			profitPercent := (profit / dbOrder.OpenBalance) * 100

			telegramMessage := fmt.Sprintf(
				"<b>💰 Ордер на продажу завершен</b>\n\n"+
					"<b>Параметры сделки:</b>\n"+
					"  Символ: %s\n"+
					"  OrderID покупки: %s\n"+
					"  OrderID продажи: %s\n"+
					"  Статус: %s\n\n"+
					"<b>Покупка:</b>\n"+
					"  Цена: %.8f\n"+
					"  Количество: %.8f\n"+
					"  Сумма: %.2f USDT\n"+
					"  Дата: %s\n\n"+
					"<b>Продажа:</b>\n"+
					"  Цена: %.8f\n"+
					"  Количество: %s\n"+
					"  Сумма: %.2f USDT\n"+
					"  Дата: %s\n\n"+
					"<b>Результат:</b>\n"+
					"  Прибыль: %.2f USDT (%.2f%%)\n"+
					"<b>Время:</b> %s",
				dbOrder.Symbol,
				dbOrder.OrderId,
				queryResult.OrderID,
				queryResult.Status,
				dbOrder.BuyPrice,
				dbOrder.Amount,
				dbOrder.OpenBalance,
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				sellPrice,
				queryResult.ExecutedQty,
				closeBalance,
				closeTime.Format("2006-01-02 15:04:05"),
				profit,
				profitPercent,
				closeTime.Format("2006-01-02 15:04:05 MST"),
			)
			_, err = u.telegramApi.Send(telegramMessage)
			if err != nil {
				fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
			}

			return nil
		}

		msg := ""
		// Получаем текущую цену пары
		currentPrice, err := u.repo.GetAvgPrice(ctx, dbOrder.Symbol)
		if err != nil {
			fmt.Printf("   ⚠️  Не удалось получить текущую цену для %s: %v\n", dbOrder.Symbol, err)
		}

		// Время открытия ордера из базы данных
		timeSinceOpen := time.Since(dbOrder.OpenDate)

		// Нижняя граница -1.5%: считаем выход за диапазон вниз только когда цена ниже этой отметки
		downLevelMinus15 := dbOrder.DownLevel * 0.985

		//Если цена вышла из диапазона
		if currentPrice.Price > dbOrder.UpLevel || currentPrice.Price < downLevelMinus15 || timeSinceOpen > 120*time.Minute {
			var reasons []string
			if currentPrice.Price > dbOrder.UpLevel {
				reasons = append(reasons, fmt.Sprintf(
					"Текущая цена вышла за диапазон вверх (%.8f > %.8f)",
					currentPrice.Price,
					dbOrder.UpLevel,
				))
			}
			if currentPrice.Price < downLevelMinus15 {
				reasons = append(reasons, fmt.Sprintf(
					"Текущая цена вышла за диапазон вниз (%.8f < %.8f)",
					currentPrice.Price,
					downLevelMinus15,
				))
			}
			if timeSinceOpen > 120*time.Minute {
				reasons = append(reasons, "Прошло больше 2 часов с момента открытия ордера")
			}
			msg = strings.Join(reasons, "\n")

			// Если текущая цена меньше цены покупки ордера и не вышла за уровень вниз (-1.5%%), прекращаем выполнение
			if currentPrice.Price < dbOrder.BuyPrice && currentPrice.Price > downLevelMinus15 {
				fmt.Printf("⚠️  Текущая цена (%.8f) меньше цены покупки ордера (%.8f), прекращаем выполнение\n",
					currentPrice.Price, dbOrder.BuyPrice)
				return nil
			}

			// Отменяем текущий лимитный ордер на продажу перед размещением маркет-ордера
			fmt.Printf("\n--- Отменяем текущий ордер на продажу ---\n")
			fmt.Printf("OrderID: %s\n", orderID)

			cancelResp, err := u.repo.CancelOrder(dbOrder.Symbol, orderID)
			if err != nil {
				fmt.Printf("❌ Ошибка при отмене ордера: %v\n", err)
				return wrap.Errorf("failed to cancel order %s: %w", orderID, err)
			}

			fmt.Printf("✅ Результат отмены ордера:\n")
			fmt.Printf("   Success: %v\n", cancelResp.Success)
			fmt.Printf("   Code: %d\n", cancelResp.Code)
			for _, result := range cancelResp.Data {
				fmt.Printf("   OrderID: %s, ErrorCode: %d, ErrorMsg: %s\n", result.OrderID, result.ErrorCode, result.ErrorMsg)
			}

			// Ждем 10 секунд, чтобы биржа обработала отмену ордера
			fmt.Println("⏳ Ожидание 10 секунд перед размещением маркет-ордера...")
			time.Sleep(10 * time.Second)

			nextOrderId, err := u.stateRepo.GetNextTradeId(ctx)
			if err != nil {
				return wrap.Errorf("failed to get next trade id: %w", err)
			}
			clientOrderId := fmt.Sprintf("Prod_order_sell_market_%d", nextOrderId)

			// Получить информацию о символе для правильного округления количества
			symbolInfo, err := u.repo.GetSymbolInfo(ctx, dbOrder.Symbol)
			if err != nil {
				fmt.Printf("❌ Ошибка получения информации о символе %s: %v\n", dbOrder.Symbol, err)
				return wrap.Errorf("failed to get symbol info for %s: %w", dbOrder.Symbol, err)
			}

			// Найти нужный символ в списке
			var symbolDetail *mexc.SymbolDetail
			for _, sym := range symbolInfo.Symbols {
				if sym.Symbol == dbOrder.Symbol {
					symbolDetail = &sym
					break
				}
			}

			if symbolDetail == nil {
				fmt.Printf("❌ Символ %s не найден в информации о бирже\n", dbOrder.Symbol)
				return wrap.Errorf("symbol %s not found in exchange info", dbOrder.Symbol)
			}

			// Округлить количество согласно baseSizePrecision
			baseSizePrecision, err := strconv.ParseFloat(symbolDetail.BaseSizePrecision, 64)
			if err != nil {
				fmt.Printf("❌ Ошибка парсинга baseSizePrecision для %s: %v\n", dbOrder.Symbol, err)
				return wrap.Errorf("failed to parse baseSizePrecision for %s: %w", dbOrder.Symbol, err)
			}

			//nolint:ineffassign,staticcheck
			roundedQuantity := dbOrder.Amount
			if baseSizePrecision == 0 {
				// Если baseSizePrecision равно 0, округлить до ближайшего целого в меньшую сторону
				roundedQuantity = math.Floor(dbOrder.Amount)
				fmt.Printf("📏 Округление количества до целого: %.8f → %.8f (baseSizePrecision: %.8f)\n",
					dbOrder.Amount, roundedQuantity, baseSizePrecision)
			} else {
				// Округлить количество до ближайшего кратного baseSizePrecision
				roundedQuantity = math.Floor(dbOrder.Amount/baseSizePrecision) * baseSizePrecision
				fmt.Printf("📏 Округление количества: %.8f → %.8f (baseSizePrecision: %.8f)\n",
					dbOrder.Amount, roundedQuantity, baseSizePrecision)
			}

			if roundedQuantity <= 0 {
				fmt.Printf("❌ Округленное количество %f недопустимо для ордера\n", roundedQuantity)
				return wrap.Errorf("rounded quantity %f is invalid for order", roundedQuantity)
			}

			placeOrderResult, err := u.repo.NewOrder(
				model.OrderParams{
					Symbol:           dbOrder.Symbol,
					Side:             order.SELL,
					OrderType:        order.MARKET,
					Quantity:         roundedQuantity,
					NewClientOrderId: clientOrderId,
				},
			)

			if err != nil {
				return wrap.Errorf("failed to place order: %w", err)
			}

			fmt.Printf("\n✅ Маркет-ордер на продажу размещен\n")
			fmt.Printf("OrderID: %s\n", placeOrderResult.OrderID)
			fmt.Printf("Symbol: %s\n", placeOrderResult.Symbol)
			fmt.Printf("Причина: %s\n", msg)

			// Сохраняем новый orderId_sell для маркет-ордера
			err = u.stateRepo.UpdateSellOrderIdTradeLog(ctx, dbOrder.ID, placeOrderResult.OrderID)
			if err != nil {
				return wrap.Errorf("failed to save sell order id: %w", err)
			}

			// Отправляем сообщение в Telegram
			marketOrderTime := helpers.NowGMT7()
			telegramMessage := fmt.Sprintf(
				"<b>🚨 Маркет-ордер на продажу размещен</b>\n\n"+
					"<b>Параметры ордера на покупку:</b>\n"+
					"  Символ: %s\n"+
					"  OrderID покупки: %s\n"+
					"  Цена покупки: %.8f\n"+
					"  Количество: %.8f\n"+
					"  Дата открытия: %s\n\n"+
					"<b>Маркет-ордер на продажу:</b>\n"+
					"  OrderID продажи: %s\n"+
					"  Количество: %.8f\n"+
					"  Текущая цена: %.8f\n\n"+
					"<b>Время:</b> %s\n"+
					"<b>Причина:</b>\n%s",
				dbOrder.Symbol,
				dbOrder.OrderId,
				dbOrder.BuyPrice,
				dbOrder.Amount,
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				placeOrderResult.OrderID,
				dbOrder.Amount,
				currentPrice.Price,
				marketOrderTime.Format("2006-01-02 15:04:05 MST"),
				msg,
			)
			_, err = u.telegramApi.Send(telegramMessage)
			if err != nil {
				fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
			}

		}
		fmt.Printf("\n--- Текущая цена %s ---\n", dbOrder.Symbol)
		fmt.Printf("Цена: %.8f\n", currentPrice.Price)
		fmt.Printf("Период усреднения: %d минут\n", currentPrice.Mins)
		if msg != "" {
			fmt.Printf("⚠️  %s\n", msg)
		}
	}

	switch queryResult.Status {
	case "NEW":

		// Время открытия ордера из базы данных
		timeSinceOpen := time.Since(dbOrder.OpenDate)
		hours := timeSinceOpen.Hours()
		minutes := timeSinceOpen.Minutes() - float64(int(hours))*60

		fmt.Printf("   ⏱️  Время с момента открытия: %.0f часов %.0f минут\n", hours, minutes)

		// Если прошло больше 2 часов, помечаем как отмененный
		if timeSinceOpen > 120*time.Minute {
			cancelResp, err := u.repo.CancelOrder(dbOrder.Symbol, dbOrder.OrderId)
			if err != nil {
				fmt.Printf("   ❌ Ошибка при отмене ордера: %v\n", err)
			} else {
				fmt.Printf("   📋 Результат отмены ордера:\n")
				fmt.Printf("      Success: %v\n", cancelResp.Success)
				fmt.Printf("      Code: %d\n", cancelResp.Code)
				for _, result := range cancelResp.Data {
					fmt.Printf("      OrderID: %s, ErrorCode: %d, ErrorMsg: %s\n", result.OrderID, result.ErrorCode, result.ErrorMsg)
				}
			}
			fmt.Printf("   ⚠️  Прошло больше 2 часов, помечаем ордер как отмененный\n")
			cancelTime := helpers.NowGMT7()
			err = u.stateRepo.UpdateCancelDateTradeLog(ctx, dbOrder.ID, cancelTime)
			if err != nil {
				return wrap.Errorf("failed to update cancel date for trade log id %d: %w", dbOrder.ID, err)
			}
			fmt.Printf("   ✅ Обновлен cancel_date в базе данных\n")

			// Отправляем сообщение в Telegram
			timeSinceOpen := time.Since(dbOrder.OpenDate)
			hours := int(timeSinceOpen.Hours())
			minutes := int(timeSinceOpen.Minutes()) % 60
			message := fmt.Sprintf(
				"<b>⏱️ Ордер отменен по времени</b>\n\n"+
					"<b>Параметры ордера:</b>\n"+
					"  Символ: %s\n"+
					"  OrderID: %s\n"+
					"  Тип ордера: %s\n"+
					"  Цена покупки: %.8f\n"+
					"  Количество: %.8f\n"+
					"  Дата открытия: %s\n\n"+
					"<b>Время:</b> %s\n"+
					"<b>Время с момента открытия:</b> %d часов %d минут\n"+
					"<b>Причина:</b> Ордер находился в статусе NEW более 2 минут\n"+
					"<b>Действие:</b> Ордер отменен и помечен как отмененный в базе данных",
				dbOrder.Symbol,
				dbOrder.OrderId,
				queryResult.Side,
				dbOrder.BuyPrice,
				dbOrder.Amount,
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				cancelTime.Format("2006-01-02 15:04:05 MST"),
				hours,
				minutes,
			)
			_, err = u.telegramApi.Send(message)
			if err != nil {
				fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
			}

		}
		return nil

	case "CANCELED", "REJECTED", "EXPIRED":
		fmt.Printf("   ⚠️  Ордер в статусе %s, помечаем как отмененный в базе данных\n", queryResult.Status)
		cancelTime := helpers.NowGMT7()
		err = u.stateRepo.UpdateCancelDateTradeLog(ctx, dbOrder.ID, cancelTime)
		if err != nil {
			fmt.Printf("   ❌ Ошибка при обновлении cancel_date: %v\n", err)
		} else {
			fmt.Printf("   ✅ Обновлен cancel_date в базе данных\n")
		}

		// Отправляем сообщение в Telegram
		reason := "Ордер отменен биржей"
		// nolint:staticcheck
		if queryResult.Status == "REJECTED" {
			reason = "Ордер отклонен биржей"
		} else if queryResult.Status == "EXPIRED" {
			reason = "Ордер истек"
		}
		message := fmt.Sprintf(
			"<b>❌ Ордер %s</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> %s\n"+
				"<b>Действие:</b> Ордер помечен как отмененный в базе данных",
			queryResult.Status,
			dbOrder.Symbol,
			queryResult.OrderID,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			cancelTime.Format("2006-01-02 15:04:05 MST"),
			reason,
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
	case "FILLED":
		fmt.Printf("   ✅ Ордер полностью исполнен (FILLED)\n")
		updateTime := helpers.NowGMT7()

		//Отправляем сообщение в Telegram
		message := fmt.Sprintf(
			"<b>✅ Ордер полностью исполнен</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер полностью исполнен на бирже\n"+
				"<b>Действие:</b> Ордер в статусе FILLED",
			dbOrder.Symbol,
			queryResult.OrderID,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			updateTime.Format("2006-01-02 15:04:05 MST"),
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}

		fmt.Printf("\n--- Размещаем ордер на продажу%s ---\n", dbOrder.Symbol)
		fmt.Printf("Цена: %.8f\n", dbOrder.UpLevel)
		fmt.Printf("Количество: %.8f\n", dbOrder.Amount)

		nextOrderId, err := u.stateRepo.GetNextTradeId(ctx)
		if err != nil {
			return wrap.Errorf("failed to get next trade id: %w", err)
		}
		clientOrderId := fmt.Sprintf("Prod_order_sell_%d", nextOrderId)

		placeOrderResult, err := u.repo.NewOrder(
			model.OrderParams{
				Symbol:           dbOrder.Symbol,
				Side:             order.SELL,
				OrderType:        order.LIMIT,
				Quantity:         dbOrder.Amount,
				QuoteOrderQty:    dbOrder.Amount,
				Price:            dbOrder.UpLevel,
				NewClientOrderId: clientOrderId,
			},
		)

		if err != nil {
			return wrap.Errorf("failed to place order: %w", err)
		}

		fmt.Printf("\nордер размещен id %s\n", placeOrderResult.OrderID)

		err = u.stateRepo.UpdateSellOrderIdTradeLog(ctx, dbOrder.ID, placeOrderResult.OrderID)
		if err != nil {
			return wrap.Errorf("failed to save sell order id: %w", err)
		}

		dealTime := helpers.NowGMT7()
		err = u.stateRepo.UpdateDealDateTradeLog(ctx, dbOrder.ID, dealTime)
		if err != nil {
			return wrap.Errorf("failed to update deal date for trade log id %d: %w", dbOrder.ID, err)
		}

		// Отправляем сообщение в Telegram о размещении ордера на продажу
		sellOrderTime := helpers.NowGMT7()
		sellMessage := fmt.Sprintf(
			"<b>💰 Ордер на продажу размещен</b>\n\n"+
				"<b>Параметры ордера на покупку:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID покупки: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Ордер на продажу:</b>\n"+
				"  OrderID продажи: %s\n"+
				"  Цена продажи: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Сумма: %.2f USDT\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер на покупку полностью исполнен (FILLED)\n"+
				"<b>Действие:</b> Размещен ордер на продажу по верхней границе",
			dbOrder.Symbol,
			dbOrder.OrderId,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			placeOrderResult.OrderID,
			dbOrder.UpLevel,
			dbOrder.Amount,
			dbOrder.UpLevel*dbOrder.Amount,
			sellOrderTime.Format("2006-01-02 15:04:05 MST"),
		)
		_, err = u.telegramApi.Send(sellMessage)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}

	case "PARTIALLY_CANCELED":
		fmt.Printf("   ⚠️  Ордер частично отменен (PARTIALLY_CANCELED)\n")
		updateTime := helpers.NowGMT7()

		// Отправляем сообщение в Telegram
		message := fmt.Sprintf(
			"<b>⚠️ Ордер частично отменен</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер частично отменен на бирже\n"+
				"<b>Действие:</b> Ордер в статусе PARTIALLY_CANCELED\n"+
				"<b>Исполнено:</b> %s / %s",
			dbOrder.Symbol,
			queryResult.OrderID,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			updateTime.Format("2006-01-02 15:04:05 MST"),
			queryResult.ExecutedQty,
			queryResult.OrigQty,
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
	default:
		updateTime := helpers.NowGMT7()

		message := fmt.Sprintf(
			"<b>⚠️ Ордер в неизвестном статусе</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер с неизвестным статусом (%s)\n"+
				"<b>Действие:</b> Ордер помечен как отмененный в базе данных",
			dbOrder.Symbol,
			dbOrder.OrderId,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			updateTime.Format("2006-01-02 15:04:05 MST"),
			queryResult.Status,
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
	}

	return nil
}
