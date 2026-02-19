package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewCheckSwapCommand(service usecase.ICheckSwap) *cli.Command {
	return &cli.Command{
		Name:  "check_swap",
		Usage: "check_swap command",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "только связки с прибылью > 1%% в формате USDT -> A -> B -> USDT",
			},
		},
		Action: func(c *cli.Context) error {
			quiet := c.Bool("quiet")
			return service.Process(context.Background(), quiet)
		},
	}
}
