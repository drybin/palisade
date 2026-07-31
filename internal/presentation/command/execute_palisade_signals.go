package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewExecutePalisadeSignalsCommand(service usecase.IExecutePalisadeSignals) *cli.Command {
	return &cli.Command{
		Name:  "execute-palisade-signals",
		Usage: "execute active palisade signals; requires --live to place orders",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "live"}},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background(), c.Bool("live"))
		},
	}
}
