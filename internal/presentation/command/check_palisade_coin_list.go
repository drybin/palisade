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
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background())
		},
	}
}
