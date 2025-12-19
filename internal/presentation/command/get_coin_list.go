package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewGetCoinListCommand(service usecase.IGetCoinList) *cli.Command {
	return &cli.Command{
		Name:  "get_coin_list",
		Usage: "get coin list command",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "enable debug output",
			},
		},
		Action: func(c *cli.Context) error {
			debug := c.Bool("debug")
			return service.Process(context.Background(), debug)
		},
	}
}
