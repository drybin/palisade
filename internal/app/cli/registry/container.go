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
	GetCoinList           *usecase.GetCoinList
	CheckPalisadeCoinList *usecase.CheckPalisadeCoinList
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

	db, err := newDbConn(config)
	if err != nil {
		return nil, wrap.Errorf("failed to connect to Postgree: %w", err)
	}

	stateRepo := registry.NewStateRepository(db)
	container := Container{
		Usecases: &Usecases{
			HelloWorld: usecase.NewHelloWorldUsecase(),
			PalisadeProcess: usecase.NewPalisadeProcessUsecase(
				mexcApi,
				mexcV2Api,
				service.NewTradingPair(),
				service.NewPalisadeLevels(mexcApi, mexcV2Api),
				service.NewByuService(mexcApi, mexcV2Api, stateRepo),
				stateRepo,
			),
			GetCoinList:           usecase.NewGetCoinListUsecase(mexcApi, stateRepo),
			CheckPalisadeCoinList: usecase.NewCheckPalisadeCoinListUsecase(mexcApi, stateRepo),
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

	return conn, nil
}
