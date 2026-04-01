package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewPalisadeProcessSellManualCommand(service *usecase.PalisadeProcessSell) *cli.Command {
	return &cli.Command{
		Name:  "process-sell-manual",
		Usage: "мониторинг сделки из trade_log_manual (как process-sell)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "опционально: YAML с trade_log_manual_id при нескольких открытых",
			},
		},
		Action: func(c *cli.Context) error {
			service.SetSellManualConfigPath(c.String("config"))
			return service.Process(context.Background())
		},
	}
}
