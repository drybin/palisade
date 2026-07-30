package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewCollectMarketDataCommand(service usecase.ICollectMarketData) *cli.Command {
	return &cli.Command{
		Name: "collect-market-data", Usage: "collect public market snapshots",
		Flags:  []cli.Flag{&cli.BoolFlag{Name: "debug"}},
		Action: func(c *cli.Context) error { return service.Process(context.Background(), c.Bool("debug")) },
	}
}
