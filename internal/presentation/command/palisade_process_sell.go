package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewPalisadeProcessSellCommand(service usecase.IPalisadeProcessSell) *cli.Command {
	return &cli.Command{
		Name:  "process-sell",
		Usage: "process sell command - check status of open orders",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background())
		},
	}
}

