package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewCheckSwapCommand(service usecase.ICheckSwap) *cli.Command {
	return &cli.Command{
		Name:  "check_swap",
		Usage: "check_swap command",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background())
		},
	}
}
