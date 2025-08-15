package registry

import (
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
            HelloWorld: usecase.NewHelloWorldUsecase(),
        },
        Clean: func() {
        },
    }
    
    return &container, nil
}
