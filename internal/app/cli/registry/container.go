package registry

import (
    repo "github.com/drybin/palisade/internal/adapter/webapi"
    "github.com/drybin/palisade/internal/app/cli/config"
    "github.com/drybin/palisade/internal/app/cli/usecase"
    "github.com/drybin/palisade/pkg/logger"
)

type Container struct {
    Logger   logger.ILogger
    Usecases *Usecases
    Clean    func()
}

type Usecases struct {
    HelloWorld      *usecase.HelloWorld
    PalisadeProcess *usecase.PalisadeProcess
}

func NewContainer(
    config *config.Config,
) (*Container, error) {
    
    container := Container{
        Usecases: &Usecases{
            HelloWorld:      usecase.NewHelloWorldUsecase(),
            PalisadeProcess: usecase.NewPalisadeProcessUsecase(repo.NewMexcWebapi(config.MexcConfig)),
        },
        Clean: func() {
        },
    }
    
    return &container, nil
}
