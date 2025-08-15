package command

import (
    "context"
    
    "github.com/drybin/palisade/internal/app/cli/usecase"
    "github.com/urfave/cli/v2"
)

func NewPalisadeProcessCommand(service usecase.IHelloWorld) *cli.Command {
    return &cli.Command{
        Name:  "palisade",
        Usage: "palisade command",
        Flags: []cli.Flag{},
        Action: func(c *cli.Context) error {
            return service.Process(context.Background())
        },
    }
}
