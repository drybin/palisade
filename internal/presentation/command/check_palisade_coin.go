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
			&cli.Float64Flag{
				Name:     "percentile",
				Aliases:  []string{"p"},
				Usage:    "Percentile for filtering klines (1-100, e.g., 95 to remove top and bottom 5%)",
				Required: false,
				Value:    0.0,
			},
		},
		Action: func(c *cli.Context) error {
			symbol := c.String("symbol")
			percentile := c.Float64("percentile")
			return service.Process(context.Background(), symbol, percentile)
		},
	}
}

