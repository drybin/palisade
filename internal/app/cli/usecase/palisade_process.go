package usecase

import (
	"context"
	"fmt"
	"math"
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

type IPalisadeProcess interface {
	Process(ctx context.Context) error
}

type PalisadeProcess struct {
	repo                  *webapi.MexcWebapi
	repoV2                *webapi.MexcV2Webapi
	telegramApi           *webapi.TelegramWebapi
	traidingPairsService  *service.TradingPair
	palisadeLevelsService *service.PalisadeLevels
	buyService            *service.ByuService
	checkerService        *service.PalisadeCheckerService
	stateRepo             repo.IStateRepository
}

func NewPalisadeProcessUsecase(
	repo *webapi.MexcWebapi,
	repoV2 *webapi.MexcV2Webapi,
	telegramApi *webapi.TelegramWebapi,
	traidingPairsService *service.TradingPair,
	palisadeLevelsService *service.PalisadeLevels,
	buyService *service.ByuService,
	checkerService *service.PalisadeCheckerService,
	stateRepo repo.IStateRepository,
) *PalisadeProcess {
	return &PalisadeProcess{
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

func (u *PalisadeProcess) Process(ctx context.Context) error {
	fmt.Println("palisade process")

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
	if cntOrdesFromTradeLog >= 1 {
		fmt.Println("Найдено 1 или больше открытых ордеров, прекращаем работу")
		return nil
	}

	usdtBalance, err := helpers.FindUSDTBalance(accountInfo.Balances)
	if err != nil {
		return wrap.Errorf("failed to find USDT balance: %w", err)
	}
	fmt.Printf("USDT Balance: %f\n", usdtBalance.Free)

	if usdtBalance.Free < 23.0 {
		fmt.Println("Balance less than 10$, stop")
		return nil
	}

	coins, err := u.stateRepo.GetCoinsToProcessTPTU(ctx, 10, 0)
	if err != nil {
		return wrap.Errorf("failed to get coins to process: %w", err)
	}
	if len(coins) == 0 {
		return wrap.Errorf("no coins to process")
	}

	coin := coins[0]

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
		return wrap.Errorf("failed to update coin %s: %w", coin.Symbol, err)
	}

	if checkResult.Skipped {
		return wrap.Errorf("coin %s was skipped: %s", coin.Symbol, checkResult.SkipReason)
	}

	// Если монета не во флете после обновления, завершаем обработку
	if !checkResult.IsFlat {
		fmt.Printf("❌ Монета %s больше не во флете после обновления\n", coin.Symbol)
		return nil
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
		return wrap.Errorf("failed to get avg price: %w", err)
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
		return nil
	}

	quantity := 2.0 / coin.Support

	// Округлить количество согласно baseSizePrecision
	baseSizePrecision, err := strconv.ParseFloat(coin.BaseSizePrecision, 64)
	if err != nil {
		return wrap.Errorf("failed to parse baseSizePrecision for %s: %w", coin.Symbol, err)
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
		return wrap.Errorf("rounded quantity %f is invalid for order", quantity)
	}

	nextOrderId, err := u.stateRepo.GetNextTradeId(ctx)
	if err != nil {
		return wrap.Errorf("failed to get next trade id: %w", err)
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
		return wrap.Errorf("failed to place order: %w", err)
	}

	fmt.Printf("\nордер размещен id %s\n", placeOrderResult.OrderID)

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
		return wrap.Errorf("failed to save trade order: %w", err)
	}

	totalBalance := usdtBalance.Free + usdtBalance.Locked
	message := fmt.Sprintf(
		"<b>📥 Покупка</b> %s · S %.8f · R %.8f · ордер <code>%s</code> · цена %.8f · кол-во %.8f · ~%.2f USDT · баланс %.2f USDT (своб %.2f · блок %.2f)",
		coin.Symbol,
		coin.Support,
		coin.Resistance,
		placeOrderResult.OrderID,
		coin.Support,
		quantity,
		coin.Support*quantity,
		totalBalance,
		usdtBalance.Free,
		usdtBalance.Locked,
	)
	_, err = u.telegramApi.Send(message)
	if err != nil {
		fmt.Printf("⚠️  Ошибка при отправке сообщения в Telegram: %v\n", err)
	}
	return nil
}
