package usecase

import (
	"context"
	"fmt"
	"math"
	"os"
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
	"gopkg.in/yaml.v3"
)

type IPalisadeProcessSell interface {
	Process(ctx context.Context) error
}

type PalisadeProcessSell struct {
	repo           *webapi.MexcWebapi
	stateRepo      repo.IStateRepository
	telegramApi    *webapi.TelegramWebapi
	useManualLog   bool
	sellConfigPath string
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
		useManualLog: false,
	}
}

// NewPalisadeProcessSellManualUsecase — process-sell-manual (таблица trade_log_manual).
func NewPalisadeProcessSellManualUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
	telegramApi *webapi.TelegramWebapi,
) *PalisadeProcessSell {
	return &PalisadeProcessSell{
		repo:         repo,
		stateRepo:    stateRepo,
		telegramApi:  telegramApi,
		useManualLog: true,
	}
}

func (u *PalisadeProcessSell) SetSellManualConfigPath(path string) {
	u.sellConfigPath = path
}

func (u *PalisadeProcessSell) clientOrderPrefix() string {
	if u.useManualLog {
		return "Manual"
	}
	return "Prod"
}

func (u *PalisadeProcessSell) persistCancel(ctx context.Context, id int, t time.Time) error {
	if u.useManualLog {
		return u.stateRepo.UpdateCancelDateTradeLogManual(ctx, id, t)
	}
	return u.stateRepo.UpdateCancelDateTradeLog(ctx, id, t)
}

func (u *PalisadeProcessSell) persistSuccess(ctx context.Context, id int, closeTime time.Time, closeBalance, sellPrice float64) error {
	if u.useManualLog {
		return u.stateRepo.UpdateSuccesTradeLogManual(ctx, id, closeTime, closeBalance, sellPrice)
	}
	return u.stateRepo.UpdateSuccesTradeLog(ctx, id, closeTime, closeBalance, sellPrice)
}

func (u *PalisadeProcessSell) nextTradeClientID(ctx context.Context) (int, error) {
	if u.useManualLog {
		return u.stateRepo.GetNextTradeIdManual(ctx)
	}
	return u.stateRepo.GetNextTradeId(ctx)
}

func (u *PalisadeProcessSell) persistSellOrderID(ctx context.Context, id int, sellID string) error {
	if u.useManualLog {
		return u.stateRepo.UpdateSellOrderIdTradeLogManual(ctx, id, sellID)
	}
	return u.stateRepo.UpdateSellOrderIdTradeLog(ctx, id, sellID)
}

func (u *PalisadeProcessSell) persistDealDate(ctx context.Context, id int, dealTime time.Time) error {
	if u.useManualLog {
		return u.stateRepo.UpdateDealDateTradeLogManual(ctx, id, dealTime)
	}
	return u.stateRepo.UpdateDealDateTradeLog(ctx, id, dealTime)
}

func (u *PalisadeProcessSell) Process(ctx context.Context) error {
	if u.useManualLog {
		fmt.Println("=== Palisade Process Sell (manual / trade_log_manual) ===")
	} else {
		fmt.Println("=== Palisade Process Sell ===")
	}

	var dbOrders []repo.TradeLog
	var err error
	if u.useManualLog {
		var dbOrder *repo.TradeLog
		if u.sellConfigPath != "" {
			b, rerr := os.ReadFile(u.sellConfigPath)
			if rerr != nil {
				return wrap.Errorf("read sell config %s: %w", u.sellConfigPath, rerr)
			}
			var y struct {
				TradeLogManualID *int `yaml:"trade_log_manual_id"`
			}
			if uerr := yaml.Unmarshal(b, &y); uerr != nil {
				return wrap.Errorf("yaml %s: %w", u.sellConfigPath, uerr)
			}
			if y.TradeLogManualID != nil {
				dbOrder, err = u.stateRepo.GetTradeLogManualById(ctx, *y.TradeLogManualID)
				if err != nil {
					return err
				}
				if dbOrder.CloseDate != nil || dbOrder.CancelDate != nil {
					return wrap.Errorf("trade_log_manual id %d уже закрыт или отменён", dbOrder.ID)
				}
			}
		}
		if dbOrder == nil {
			open, gerr := u.stateRepo.GetOpenOrdersManual(ctx)
			if gerr != nil {
				return gerr
			}
			if len(open) == 0 {
				fmt.Println("Нет открытых записей в trade_log_manual")
				return nil
			}
			if len(open) > 1 {
				return wrap.Errorf("открыто %d manual-сделок: укажите trade_log_manual_id в YAML (--config)", len(open))
			}
			dbOrder = &open[0]
		}
		dbOrders = []repo.TradeLog{*dbOrder}
	} else {
		dbOrders, err = u.stateRepo.GetOpenOrders(ctx)
		if err != nil {
			return wrap.Errorf("failed to get open orders from database: %w", err)
		}
		if len(dbOrders) == 0 {
			fmt.Println("Нет открытых ордеров в базе данных")
			return nil
		}
	}

	dbOrder := dbOrders[0]
	if u.useManualLog {
		fmt.Printf("\n--- trade_log_manual id=%d ---\n\n", dbOrder.ID)
	} else {
		fmt.Printf("\nНайден открытый ордер в базе данных\n\n")
		fmt.Printf("--- Ордер ---\n")
		fmt.Printf("ID в БД: %d\n", dbOrder.ID)
	}
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

	queryResult, err := u.repo.GetOrderQuery(dbOrder.Symbol, orderID)
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
		err = u.persistCancel(ctx, dbOrder.ID, cancelTime)
		if err != nil {
			return wrap.Errorf("failed to update cancel date for trade log id %d: %w", dbOrder.ID, err)
		}
		fmt.Printf("✅ Обновлен cancel_date в базе данных\n")

		// Отправляем сообщение в Telegram
		message := fmt.Sprintf(
			"<b>⚠️ Нет на бирже</b> %s · <code>%s</code> · покупка %s×%s · открыт %s · БД cancel · %s · нет среди открытых (исполнен/отменён?)",
			dbOrder.Symbol,
			dbOrder.OrderId,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
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

			err = u.persistSuccess(ctx, dbOrder.ID, closeTime, closeBalance, sellPrice)
			if err != nil {
				return wrap.Errorf("failed to update success trade log for id %d: %w", dbOrder.ID, err)
			}
			fmt.Printf("✅ Обновлен close_date в базе данных\n")

			// Отправляем сообщение в Telegram
			buyCost := dbOrder.BuyPrice * dbOrder.Amount
			profit := closeBalance - buyCost
			profitPercent := (profit / buyCost) * 100

			telegramMessage := fmt.Sprintf(
				"<b>💰 Продажа завершена</b> %s · P/L %s USDT (%s%%) · buy <code>%s</code> sell <code>%s</code> · покупка %s×%s · баланс на откр %s USDT (%s) · продажа %s×%s = %s USDT (%s) · статус %s · %s",
				dbOrder.Symbol,
				helpers.FormatFloatSignTrimZeros(profit),
				helpers.FormatFloatSignTrimZeros(profitPercent),
				dbOrder.OrderId,
				queryResult.OrderID,
				helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
				helpers.FormatFloatTrimZeros(dbOrder.Amount),
				helpers.FormatFloatTrimZeros(dbOrder.OpenBalance),
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				helpers.FormatFloatTrimZeros(sellPrice),
				helpers.TrimDecimalZeros(queryResult.ExecutedQty),
				helpers.FormatFloatTrimZeros(closeBalance),
				closeTime.Format("2006-01-02 15:04:05"),
				queryResult.Status,
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

			// Если единственный триггер — время (цена в диапазоне), не продавать в минус
			timeOnlyTrigger := timeSinceOpen > 120*time.Minute &&
				currentPrice.Price <= dbOrder.UpLevel &&
				currentPrice.Price >= downLevelMinus15
			if timeOnlyTrigger && currentPrice.Price <= dbOrder.BuyPrice {
				fmt.Printf("⚠️  Триггер только по времени: текущая цена (%.8f) меньше цены покупки (%.8f), не продаём в минус\n",
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

			nextOrderId, err := u.nextTradeClientID(ctx)
			if err != nil {
				return wrap.Errorf("failed to get next trade id: %w", err)
			}
			clientOrderId := fmt.Sprintf("%s_order_sell_market_%d", u.clientOrderPrefix(), nextOrderId)

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
			err = u.persistSellOrderID(ctx, dbOrder.ID, placeOrderResult.OrderID)
			if err != nil {
				return wrap.Errorf("failed to save sell order id: %w", err)
			}

			// Отправляем сообщение в Telegram
			marketOrderTime := helpers.NowGMT7()
			reasonOneLine := strings.ReplaceAll(strings.ReplaceAll(msg, "\n", " · "), "  ", " ")
			telegramMessage := fmt.Sprintf(
				"<b>🚨 Маркет-продажа</b> %s · sell <code>%s</code> · qty %s · цена ~%s · buy <code>%s</code> · открыт %s · %s · %s",
				dbOrder.Symbol,
				placeOrderResult.OrderID,
				helpers.FormatFloatTrimZeros(dbOrder.Amount),
				helpers.FormatFloatTrimZeros(currentPrice.Price),
				dbOrder.OrderId,
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				marketOrderTime.Format("2006-01-02 15:04:05 MST"),
				reasonOneLine,
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
			err = u.persistCancel(ctx, dbOrder.ID, cancelTime)
			if err != nil {
				return wrap.Errorf("failed to update cancel date for trade log id %d: %w", dbOrder.ID, err)
			}
			fmt.Printf("   ✅ Обновлен cancel_date в базе данных\n")

			// Отправляем сообщение в Telegram
			timeSinceOpen := time.Since(dbOrder.OpenDate)
			hours := int(timeSinceOpen.Hours())
			minutes := int(timeSinceOpen.Minutes()) % 60
			message := fmt.Sprintf(
				"<b>⏱️ Покупка отменена по времени</b> %s · <code>%s</code> · %s · %s×%s · открыт %s · висел %dч %dм · >2ч в NEW · БД cancel · %s",
				dbOrder.Symbol,
				dbOrder.OrderId,
				queryResult.Side,
				helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
				helpers.FormatFloatTrimZeros(dbOrder.Amount),
				dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
				hours,
				minutes,
				cancelTime.Format("2006-01-02 15:04:05 MST"),
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
		err = u.persistCancel(ctx, dbOrder.ID, cancelTime)
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
			"<b>❌ Покупка %s</b> %s · <code>%s</code> · %s×%s · открыт %s · %s · БД cancel · %s",
			queryResult.Status,
			dbOrder.Symbol,
			queryResult.OrderID,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			reason,
			cancelTime.Format("2006-01-02 15:04:05 MST"),
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
			"<b>✅ Покупка FILLED</b> %s · <code>%s</code> · %s×%s · открыт %s · далее лимит на продажу · %s",
			dbOrder.Symbol,
			queryResult.OrderID,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
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

		nextOrderId, err := u.nextTradeClientID(ctx)
		if err != nil {
			return wrap.Errorf("failed to get next trade id: %w", err)
		}
		clientOrderId := fmt.Sprintf("%s_order_sell_%d", u.clientOrderPrefix(), nextOrderId)

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

		err = u.persistSellOrderID(ctx, dbOrder.ID, placeOrderResult.OrderID)
		if err != nil {
			return wrap.Errorf("failed to save sell order id: %w", err)
		}

		dealTime := helpers.NowGMT7()
		err = u.persistDealDate(ctx, dbOrder.ID, dealTime)
		if err != nil {
			return wrap.Errorf("failed to update deal date for trade log id %d: %w", dbOrder.ID, err)
		}

		// Отправляем сообщение в Telegram о размещении ордера на продажу
		sellOrderTime := helpers.NowGMT7()
		sellMessage := fmt.Sprintf(
			"<b>💸 Лимит на продажу</b> %s · sell <code>%s</code> @ %s · qty %s · ~%s USDT · buy <code>%s</code> · %s×%s · открыт %s · %s",
			dbOrder.Symbol,
			placeOrderResult.OrderID,
			helpers.FormatFloatTrimZeros(dbOrder.UpLevel),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
			helpers.FormatFloatTrimZeros(dbOrder.UpLevel*dbOrder.Amount),
			dbOrder.OrderId,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
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
			"<b>⚠️ Частичная отмена покупки</b> %s · <code>%s</code> · %s×%s · открыт %s · исполнено %s / %s · %s",
			dbOrder.Symbol,
			queryResult.OrderID,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			helpers.TrimDecimalZeros(queryResult.ExecutedQty),
			helpers.TrimDecimalZeros(queryResult.OrigQty),
			updateTime.Format("2006-01-02 15:04:05 MST"),
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
	default:
		updateTime := helpers.NowGMT7()

		message := fmt.Sprintf(
			"<b>⚠️ Неизвестный статус покупки</b> %s · <code>%s</code> · биржа status=%s · %s×%s · открыт %s · БД cancel · %s",
			dbOrder.Symbol,
			dbOrder.OrderId,
			queryResult.Status,
			helpers.FormatFloatTrimZeros(dbOrder.BuyPrice),
			helpers.FormatFloatTrimZeros(dbOrder.Amount),
			dbOrder.OpenDate.Format("2006-01-02 15:04:05"),
			updateTime.Format("2006-01-02 15:04:05 MST"),
		)
		_, err = u.telegramApi.Send(message)
		if err != nil {
			fmt.Printf("⚠️  Не удалось отправить сообщение в Telegram: %v\n", err)
		}
	}

	return nil
}
