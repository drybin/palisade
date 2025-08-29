package registry

import (
    repo "github.com/drybin/palisade/internal/adapter/webapi"
    "github.com/drybin/palisade/internal/app/cli/config"
    "github.com/drybin/palisade/internal/app/cli/usecase"
    "github.com/drybin/palisade/pkg/logger"
    "mexc-sdk/mexcsdk"
    "github.com/go-resty/resty/v2"
)

type Container struct {
    Logger   logger.ILogger
    Usecases *Usecases
    MexcSpot mexcsdk.Spot
    Clean    func()
}

type Usecases struct {
    HelloWorld      *usecase.HelloWorld
    PalisadeProcess *usecase.PalisadeProcess
}

func NewContainer(
    config *config.Config,
) (*Container, error) {
    
    httpClient := resty.New()
    httpClient.SetBaseURL(config.MexcConfig.BaseUrl)
    httpClient.SetHeader("Content-Type", "application/json")
    httpClient.SetHeader("X-MEXC-APIKEY", config.MexcConfig.ApiKey)
    
    // Создаем экземпляр mexc Spot SDK
    mexcSpot := mexcsdk.NewSpot(&config.MexcConfig.ApiKey, &config.MexcConfig.Secret)
    
    container := Container{
        Usecases: &Usecases{
            HelloWorld: usecase.NewHelloWorldUsecase(),
            PalisadeProcess: usecase.NewPalisadeProcessUsecase(
                repo.NewMexcWebapi(httpClient, config.MexcConfig),
            ),
        },
        MexcSpot: mexcSpot,
        Clean: func() {
        },
    }
    
    return &container, nil
}
