package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewPalisadeProcessManualCommand(service *usecase.PalisadeProcessManual) *cli.Command {
	return &cli.Command{
		Name:  "process-manual",
		Usage: "лимитная покупка по YAML → trade_log_manual",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "process_manual.yaml",
				Usage:   "путь к YAML (symbol, support, resistance, …)",
			},
		},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background(), c.String("config"))
		},
	}
}
