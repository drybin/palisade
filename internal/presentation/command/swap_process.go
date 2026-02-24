package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewSwapProcessCommand(service usecase.ISwapProcess) *cli.Command {
	return &cli.Command{
		Name:  "swap-process",
		Usage: "найти связку USDT->A->B->USDT с прибылью >1% и исполнить лимитными ордерами по очереди",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background())
		},
	}
}
