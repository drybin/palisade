package usecase

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
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

// ProcessManualConfig задаётся в YAML для команды process-manual.
type ProcessManualConfig struct {
	Symbol                string  `yaml:"symbol"`
	Support               float64 `yaml:"support"`
	Resistance            float64 `yaml:"resistance"`
	QuoteUsdt             float64 `yaml:"quote_usdt"`
	SkipPriceRangeCheck   bool    `yaml:"skip_price_range_check"`
}

type PalisadeProcessManual struct {
	repo        *webapi.MexcWebapi
	stateRepo   repo.IStateRepository
	telegramApi *webapi.TelegramWebapi
}

func NewPalisadeProcessManualUsecase(
	repo *webapi.MexcWebapi,
	stateRepo repo.IStateRepository,
	telegramApi *webapi.TelegramWebapi,
) *PalisadeProcessManual {
	return &PalisadeProcessManual{
		repo:        repo,
		stateRepo:   stateRepo,
		telegramApi: telegramApi,
	}
}

func loadProcessManualConfig(path string) (*ProcessManualConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, wrap.Errorf("read config %s: %w", path, err)
	}
	var c ProcessManualConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, wrap.Errorf("yaml %s: %w", path, err)
	}
	if c.Symbol == "" {
		return nil, wrap.Errorf("config: symbol is required")
	}
	if c.Support <= 0 || c.Resistance <= 0 {
		return nil, wrap.Errorf("config: support and resistance must be > 0")
	}
	if c.QuoteUsdt <= 0 {
		c.QuoteUsdt = 2
	}
	return &c, nil
}

func findSymbolDetailManual(info *mexc.SymbolInfo, symbol string) *mexc.SymbolDetail {
	for i := range info.Symbols {
		if info.Symbols[i].Symbol == symbol {
			return &info.Symbols[i]
		}
	}
	return nil
}

func (u *PalisadeProcessManual) Process(ctx context.Context, configPath string) error {
	fmt.Println("=== Palisade process-manual (trade_log_manual) ===")

	cfg, err := loadProcessManualConfig(configPath)
	if err != nil {
		return err
	}

	accountInfo, err := u.repo.GetBalance(ctx)
	if err != nil {
		return wrap.Errorf("failed to get balance: %w", err)
	}

	openManual, err := u.stateRepo.GetOpenOrdersManual(ctx)
	if err != nil {
		return err
	}
	if len(openManual) >= 1 {
		fmt.Println("Есть открытые записи в trade_log_manual, прекращаем работу")
		return nil
	}

	usdtBalance, err := helpers.FindUSDTBalance(accountInfo.Balances)
	if err != nil {
		return wrap.Errorf("failed to find USDT balance: %w", err)
	}
	if usdtBalance.Free < 23.0 {
		fmt.Println("Balance less than 23 USDT, stop")
		return nil
	}

	symbolInfo, err := u.repo.GetSymbolInfo(ctx, cfg.Symbol)
	if err != nil {
		return wrap.Errorf("GetSymbolInfo %s: %w", cfg.Symbol, err)
	}
	detail := findSymbolDetailManual(symbolInfo, cfg.Symbol)
	if detail == nil {
		return wrap.Errorf("symbol %s not found in exchange info", cfg.Symbol)
	}
	baseSizePrecision, err := strconv.ParseFloat(detail.BaseSizePrecision, 64)
	if err != nil {
		return wrap.Errorf("baseSizePrecision %s: %w", cfg.Symbol, err)
	}

	if !cfg.SkipPriceRangeCheck {
		avg, err := u.repo.GetAvgPrice(ctx, cfg.Symbol)
		if err != nil {
			return wrap.Errorf("GetAvgPrice: %w", err)
		}
		if avg.Price < cfg.Support || avg.Price > cfg.Resistance {
			fmt.Printf("Цена %.8f вне диапазона %.8f .. %.8f (skip_price_range_check: true чтобы игнорировать)\n",
				avg.Price, cfg.Support, cfg.Resistance)
			return nil
		}
		fmt.Printf("Текущая цена %.8f в диапазоне support..resistance\n", avg.Price)
	}

	quantity := cfg.QuoteUsdt / cfg.Support
	if baseSizePrecision == 0 {
		quantity = math.Floor(quantity)
	} else {
		quantity = math.Floor(quantity/baseSizePrecision) * baseSizePrecision
	}
	if quantity <= 0 {
		return wrap.Errorf("rounded quantity invalid for %s", cfg.Symbol)
	}

	nextID, err := u.stateRepo.GetNextTradeIdManual(ctx)
	if err != nil {
		return err
	}
	clientOrderID := fmt.Sprintf("Manual_buy_%d", nextID)

	fmt.Printf("Размещаем лимит BUY %s @ %.8f qty %.8f\n", cfg.Symbol, cfg.Support, quantity)
	placeOrderResult, err := u.repo.NewOrder(model.OrderParams{
		Symbol:           cfg.Symbol,
		Side:             order.BUY,
		OrderType:        order.LIMIT,
		Quantity:         quantity,
		QuoteOrderQty:    quantity,
		Price:            cfg.Support,
		NewClientOrderId: clientOrderID,
	})
	if err != nil {
		return wrap.Errorf("place order: %w", err)
	}

	_, err = u.stateRepo.SaveTradeLogManual(ctx, repo.SaveTradeLogParams{
		OpenDate:    time.Now(),
		OpenBalance: usdtBalance.Free,
		Symbol:      cfg.Symbol,
		BuyPrice:    cfg.Support,
		Amount:      quantity,
		OrderId:     placeOrderResult.OrderID,
		UpLevel:     cfg.Resistance,
		DownLevel:   cfg.Support,
	})
	if err != nil {
		return err
	}

	totalBalance := usdtBalance.Free + usdtBalance.Locked
	msg := fmt.Sprintf(
		"<b>📥 Покупка [manual]</b> %s · S %s · R %s · ордер <code>%s</code> · цена %s · кол-во %s · ~%s USDT · баланс %s USDT (своб %s · блок %s)",
		cfg.Symbol,
		helpers.FormatFloatTrimZeros(cfg.Support),
		helpers.FormatFloatTrimZeros(cfg.Resistance),
		placeOrderResult.OrderID,
		helpers.FormatFloatTrimZeros(cfg.Support),
		helpers.FormatFloatTrimZeros(quantity),
		helpers.FormatFloatTrimZeros(cfg.Support*quantity),
		helpers.FormatFloatTrimZeros(totalBalance),
		helpers.FormatFloatTrimZeros(usdtBalance.Free),
		helpers.FormatFloatTrimZeros(usdtBalance.Locked),
	)
	if _, err := u.telegramApi.Send(msg); err != nil {
		fmt.Printf("⚠️  Telegram: %v\n", err)
	}
	return nil
}
