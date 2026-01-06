package registry

import (
	"context"
	"mexc-sdk/mexcsdk"

	registry "github.com/drybin/palisade/internal/adapter/pg"
	repo "github.com/drybin/palisade/internal/adapter/webapi"
	"github.com/drybin/palisade/internal/app/cli/config"
	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/drybin/palisade/internal/domain/service"
	"github.com/drybin/palisade/pkg/logger"
	"github.com/drybin/palisade/pkg/wrap"
	"github.com/go-resty/resty/v2"
	"github.com/jackc/pgx/v5"
)

type Container struct {
	Logger   logger.ILogger
	Usecases *Usecases
	MexcSpot mexcsdk.Spot
	Clean    func()
}

type Usecases struct {
	HelloWorld            *usecase.HelloWorld
	PalisadeProcess       *usecase.PalisadeProcess
	PalisadeProcessMulti  *usecase.PalisadeProcessMulti
	PalisadeProcessSell   *usecase.PalisadeProcessSell
	GetCoinList           *usecase.GetCoinList
	CheckPalisadeCoinList *usecase.CheckPalisadeCoinList
	CheckPalisadeCoin     *usecase.CheckPalisadeCoin
}

func NewContainer(
	config *config.Config,
) (*Container, error) {

	httpClient := resty.New()
	httpClient.SetBaseURL(config.MexcConfig.BaseUrl)
	httpClient.SetHeader("Content-Type", "application/json")
	httpClient.SetHeader("X-MEXC-APIKEY", config.MexcConfig.ApiKey)

	// Создаем экземпляр spot Spot SDK
	mexcSpot := mexcsdk.NewSpot(&config.MexcConfig.ApiKey, &config.MexcConfig.Secret)

	mexcApi := repo.NewMexcWebapi(httpClient, mexcSpot, config.MexcConfig)
	mexcV2Api := repo.NewMexcV2Webapi(httpClient, config.MexcConfig)

	// Создаем отдельный HTTP клиент для Telegram API
	telegramHttpClient := resty.New()
	telegramApi := repo.NewTelegramWebapi(telegramHttpClient, config.TgConfig.BotToken, config.TgConfig.ChatId)

	db, err := newDbConn(config)
	if err != nil {
		return nil, wrap.Errorf("failed to connect to Postgree: %w", err)
	}

	stateRepo := registry.NewStateRepository(db)
	
	// Создаем сервисы
	palisadeCheckerService := service.NewPalisadeCheckerService(mexcApi, stateRepo)
	
	container := Container{
		Usecases: &Usecases{
			HelloWorld: usecase.NewHelloWorldUsecase(),
		PalisadeProcess: usecase.NewPalisadeProcessUsecase(
			mexcApi,
			mexcV2Api,
			telegramApi,
			service.NewTradingPair(),
			service.NewPalisadeLevels(mexcApi, mexcV2Api),
			service.NewByuService(mexcApi, mexcV2Api, stateRepo),
			palisadeCheckerService,
			stateRepo,
		),
		PalisadeProcessMulti: usecase.NewPalisadeProcessMultiUsecase(
			mexcApi,
			mexcV2Api,
			telegramApi,
			service.NewTradingPair(),
			service.NewPalisadeLevels(mexcApi, mexcV2Api),
			service.NewByuService(mexcApi, mexcV2Api, stateRepo),
			palisadeCheckerService,
			stateRepo,
		),
		PalisadeProcessSell:   usecase.NewPalisadeProcessSellUsecase(mexcApi, stateRepo, telegramApi),
		GetCoinList:           usecase.NewGetCoinListUsecase(mexcApi, stateRepo),
		CheckPalisadeCoinList: usecase.NewCheckPalisadeCoinListUsecase(palisadeCheckerService, stateRepo),
		CheckPalisadeCoin:     usecase.NewCheckPalisadeCoinUsecase(palisadeCheckerService, stateRepo),
		},
		MexcSpot: mexcSpot,
		Clean: func() {
		},
	}

	return &container, nil
}

func newDbConn(config *config.Config) (*pgx.Conn, error) {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, config.PostgreeDsn)
	if err != nil {
		return nil, err
	}

	// Устанавливаем часовой пояс сессии на GMT+7
	// Это влияет на то, как PostgreSQL будет отображать TIMESTAMPTZ при чтении
	_, err = conn.Exec(ctx, "SET timezone = 'Asia/Bangkok'")
	if err != nil {
		_ = conn.Close(ctx)
		return nil, wrap.Errorf("failed to set timezone: %w", err)
	}

	return conn, nil
}
