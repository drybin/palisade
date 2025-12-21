package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/helpers"
	"github.com/drybin/palisade/internal/domain/model"
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
	exchangeOrders, err := u.repo.GetOpenOrders(ctx, model.OrderParams{
		Symbol: dbOrder.Symbol,
	})
	if err != nil {
		return wrap.Errorf("ошибка при получении ордеров с биржи для %s: %w", dbOrder.Symbol, err)
	}

	if exchangeOrders == nil || len(*exchangeOrders) == 0 {
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
		_, _ = u.telegramApi.Send(message)
		return nil
	}

	// Ищем наш ордер среди ордеров с биржи по OrderId
	found := false
	for _, exchangeOrder := range *exchangeOrders {
		// Сравниваем OrderId (оба теперь строки)
		if exchangeOrder.OrderID == dbOrder.OrderId {
			found = true
			fmt.Printf("✅ Статус: Ордер найден на бирже\n")
			fmt.Printf("   Статус на бирже: %s\n", exchangeOrder.Status)
			fmt.Printf("   Тип: %s\n", exchangeOrder.Type)
			fmt.Printf("   Сторона: %s\n", exchangeOrder.Side)
			fmt.Printf("   Цена: %s\n", exchangeOrder.Price)
			fmt.Printf("   Исходное количество: %s\n", exchangeOrder.OrigQty)
			fmt.Printf("   Исполнено: %s\n", exchangeOrder.ExecutedQty)
			fmt.Printf("   Накопленная сумма: %s\n", exchangeOrder.CummulativeQuoteQty)
			fmt.Printf("   Время создания: %d\n", exchangeOrder.Time)
			fmt.Printf("   Время обновления: %d\n", exchangeOrder.UpdateTime)
			if exchangeOrder.CancelReason != nil {
				fmt.Printf("   Причина отмены: %s\n", *exchangeOrder.CancelReason)
			}

			// Если статус NEW, проверяем время с момента открытия
			if exchangeOrder.Status == "NEW" {
				// Время открытия ордера из базы данных
				timeSinceOpen := time.Since(dbOrder.OpenDate)
				hours := timeSinceOpen.Hours()
				minutes := timeSinceOpen.Minutes() - float64(int(hours))*60

				fmt.Printf("   ⏱️  Время с момента открытия: %.0f часов %.0f минут\n", hours, minutes)

				// Если прошло больше 2 часов, помечаем как отмененный
				if timeSinceOpen > 2*time.Minute {
					cancelResp, err := u.repo.CancelOrder(exchangeOrder.Symbol, exchangeOrder.OrderID)
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
							"  Цена покупки: %.8f\n"+
							"  Количество: %.8f\n"+
							"  Дата открытия: %s\n\n"+
							"<b>Время:</b> %s\n"+
							"<b>Время с момента открытия:</b> %d часов %d минут\n"+
							"<b>Причина:</b> Ордер находился в статусе NEW более 2 минут\n"+
							"<b>Действие:</b> Ордер отменен и помечен как отмененный в базе данных",
						dbOrder.Symbol,
						exchangeOrder.OrderID,
						dbOrder.BuyPrice,
						dbOrder.Amount,
						dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
						cancelTime.Format("2006-01-02 15:04:05 MST"),
						hours,
						minutes,
					)
					_, _ = u.telegramApi.Send(message)
					return nil
				}
			}

			queryResult, err := u.repo.GetOrderQuery(exchangeOrder.Symbol, exchangeOrder.OrderID)

			if err != nil {
				fmt.Printf("   ❌ Ошибка при запросе информации об ордере: %v\n", err)
			} else {
				fmt.Printf("   📋 Информация об ордере:\n")
				fmt.Printf("      Статус: %s\n", queryResult.Status)
				fmt.Printf("      Исполнено: %s / %s\n", queryResult.ExecutedQty, queryResult.OrigQty)

				// Проверяем статус ордера
				status := queryResult.Status
				updateTime := helpers.NowGMT7()
				switch status {
				case "CANCELED", "REJECTED", "EXPIRED":
					fmt.Printf("   ⚠️  Ордер в статусе %s, помечаем как отмененный в базе данных\n", status)
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
					if status == "REJECTED" {
						reason = "Ордер отклонен биржей"
					} else if status == "EXPIRED" {
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
						status,
						dbOrder.Symbol,
						queryResult.OrderID,
						dbOrder.BuyPrice,
						dbOrder.Amount,
						dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
						cancelTime.Format("2006-01-02 15:04:05 MST"),
						reason,
					)
					_, _ = u.telegramApi.Send(message)
				case "FILLED":
					fmt.Printf("   ✅ Ордер полностью исполнен (FILLED)\n")

					// Отправляем сообщение в Telegram
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
					_, _ = u.telegramApi.Send(message)
				case "PARTIALLY_CANCELED":
					fmt.Printf("   ⚠️  Ордер частично отменен (PARTIALLY_CANCELED)\n")

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
					_, _ = u.telegramApi.Send(message)
				}
			}

			break
		}
	}

	if !found {
		fmt.Printf("⚠️  Статус: Ордер с OrderId %s не найден среди открытых ордеров на бирже\n", dbOrder.OrderId)
		fmt.Printf("   Всего открытых ордеров на бирже для %s: %d\n", dbOrder.Symbol, len(*exchangeOrders))
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
			"<b>⚠️ Ордер не найден среди открытых</b>\n\n"+
				"<b>Параметры ордера:</b>\n"+
				"  Символ: %s\n"+
				"  OrderID: %s\n"+
				"  Цена покупки: %.8f\n"+
				"  Количество: %.8f\n"+
				"  Дата открытия: %s\n\n"+
				"<b>Время:</b> %s\n"+
				"<b>Причина:</b> Ордер с указанным OrderID не найден среди открытых ордеров на бирже (всего открытых: %d)\n"+
				"<b>Действие:</b> Ордер помечен как отмененный в базе данных",
			dbOrder.Symbol,
			dbOrder.OrderId,
			dbOrder.BuyPrice,
			dbOrder.Amount,
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			cancelTime.Format("2006-01-02 15:04:05 MST"),
			len(*exchangeOrders),
		)
		_, _ = u.telegramApi.Send(message)
	}

	return nil
}
