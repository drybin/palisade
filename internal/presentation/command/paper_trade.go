package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewPaperTradeCommand(service usecase.IPaperTrade) *cli.Command {
	return &cli.Command{
		Name:  "paper-palisade-signals",
		Usage: "simulate palisade signal execution without placing orders",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "debug"}},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background(), c.Bool("debug"))
		},
	}
}
