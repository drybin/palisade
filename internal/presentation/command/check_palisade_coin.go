package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewCheckPalisadeCoinCommand(service usecase.ICheckPalisadeCoin) *cli.Command {
	return &cli.Command{
		Name:  "check_palisade_coin",
		Usage: "check palisade coin command",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "symbol",
				Aliases:  []string{"s"},
				Usage:    "Symbol of the coin to check (e.g., BTCUSDT)",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			symbol := c.String("symbol")
			return service.Process(context.Background(), symbol)
		},
	}
}

