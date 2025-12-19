package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewCheckPalisadeCoinListCommand(service usecase.ICheckPalisadeCoinList) *cli.Command {
	return &cli.Command{
		Name:  "check_palisade_coin_list",
		Usage: "check palisade coin list command",
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
