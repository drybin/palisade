package usecase

import (
	"context"
	"fmt"

	"github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/domain/model"
	"github.com/drybin/palisade/internal/domain/repo"
	"github.com/drybin/palisade/pkg/wrap"
)

type IReconcileOrders interface {
	Process(context.Context) error
}

type ReconcileOrders struct {
	api       *webapi.MexcWebapi
	stateRepo repo.IStateRepository
	telegram  *webapi.TelegramWebapi
}

func NewReconcileOrdersUsecase(api *webapi.MexcWebapi, stateRepo repo.IStateRepository, telegram *webapi.TelegramWebapi) *ReconcileOrders {
	return &ReconcileOrders{api: api, stateRepo: stateRepo, telegram: telegram}
}

// Process never creates an order. It only resolves intents that may have lost
// the exchange response and links them to the local trade log.
func (u *ReconcileOrders) Process(ctx context.Context) error {
	releaseLock, acquired, err := acquireTradingLock(ctx, u.stateRepo)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer releaseLock()

	intents, err := u.stateRepo.ListRecoverableOrderIntents(ctx)
	if err != nil {
		return err
	}
	if len(intents) == 0 {
		fmt.Println("Неразрешённых намерений ордеров нет")
		return nil
	}

	for _, intent := range intents {
		if err := u.reconcileIntent(ctx, intent); err != nil {
			return err
		}
	}
	return nil
}

func (u *ReconcileOrders) reconcileIntent(ctx context.Context, intent repo.OrderIntent) error {
	orderInfo, err := u.api.GetOrderQueryByClientID(intent.Symbol, intent.ClientOrderID)
	if err != nil {
		return wrap.Errorf("find intent %s: %w", intent.ClientOrderID, err)
	}
	if orderInfo == nil && intent.ExchangeOrderID != "" {
		orderInfo, err = u.api.GetOrderQuery(intent.Symbol, intent.ExchangeOrderID)
		if err != nil {
			return wrap.Errorf("query intent %s by exchange id: %w", intent.ClientOrderID, err)
		}
	}
	if orderInfo == nil {
		openOrders, openErr := u.api.GetOpenOrders(ctx, model.OrderParams{Symbol: intent.Symbol})
		if openErr != nil {
			return wrap.Errorf("search open orders for intent %s: %w", intent.ClientOrderID, openErr)
		}
		for _, candidate := range *openOrders {
			if candidate.ClientOrderID == intent.ClientOrderID {
				orderInfo = &candidate
				break
			}
		}
	}
	if orderInfo == nil {
		reason := "order not found by clientOrderId or exchange order id"
		if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "RECOVERY_REQUIRED", intent.ExchangeOrderID, intent.ExecutedQuantity, intent.CumulativeQuoteQty, reason); err != nil {
			return err
		}
		u.notify(fmt.Sprintf("<b>⚠️ RECOVERY_REQUIRED</b> %s · <code>%s</code> · ордер не найден", intent.Symbol, intent.ClientOrderID))
		fmt.Printf("%s: ордер не найден, требуется ручная проверка\n", intent.ClientOrderID)
		return nil
	}

	executed, quote, err := orderFill(orderInfo)
	if err != nil {
		return err
	}
	if orderInfo.Status == "NEW" || orderInfo.Status == "PARTIALLY_FILLED" {
		fmt.Printf("Восстановлен активный ордер: %s %s status=%s\n", intent.Symbol, intent.ClientOrderID, orderInfo.Status)
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, "ACKNOWLEDGED", orderInfo.OrderID, executed, quote, ""); err != nil {
		return err
	}

	tradeID := intent.TradeID
	if tradeID == 0 {
		openTrades, err := u.stateRepo.GetOpenOrders(ctx)
		if err != nil {
			return err
		}
		for _, trade := range openTrades {
			if trade.OrderId == orderInfo.OrderID {
				tradeID = trade.ID
				break
			}
		}
	}

	if tradeID == 0 && intent.Side == "BUY" {
		amount := intent.Quantity
		buyPrice := intent.Price
		if executed > 0 {
			amount = executed
		}
		if quote > 0 && executed > 0 {
			buyPrice = quote / executed
		}
		trade, err := u.stateRepo.SaveTradeLog(ctx, repo.SaveTradeLogParams{
			OpenDate:    intent.CreatedAt,
			OpenBalance: intent.OpenBalance,
			Symbol:      intent.Symbol,
			BuyPrice:    buyPrice,
			Amount:      amount,
			OrderId:     orderInfo.OrderID,
			UpLevel:     intent.TargetPrice,
			DownLevel:   intent.Price,
		})
		if err != nil {
			return wrap.Errorf("recreate trade log for intent %s: %w", intent.ClientOrderID, err)
		}
		tradeID = trade.ID
	}

	if tradeID > 0 {
		if err := u.stateRepo.UpdateOrderIntentTradeID(ctx, intent.ID, tradeID); err != nil {
			return err
		}
		if intent.Side == "SELL" {
			if err := u.stateRepo.UpdateSellOrderIdTradeLog(ctx, tradeID, orderInfo.OrderID); err != nil {
				return err
			}
		}
	}
	status := "LINKED"
	if tradeID == 0 {
		status = "RECOVERY_REQUIRED"
	}
	if err := u.stateRepo.UpdateOrderIntent(ctx, intent.ID, status, orderInfo.OrderID, executed, quote, ""); err != nil {
		return err
	}
	if status == "RECOVERY_REQUIRED" {
		return wrap.Errorf("intent %s found on exchange but has no local trade", intent.ClientOrderID)
	}
	fmt.Printf("Восстановлен intent %s: exchange=%s trade_log=%d status=%s\n", intent.ClientOrderID, orderInfo.OrderID, tradeID, orderInfo.Status)
	return nil
}

func (u *ReconcileOrders) notify(message string) {
	if u.telegram == nil || !u.telegram.Configured() {
		return
	}
	if _, err := u.telegram.Send(message); err != nil {
		fmt.Printf("Telegram: %v\n", err)
	}
}
