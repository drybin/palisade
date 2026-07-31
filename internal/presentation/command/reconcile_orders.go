package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewReconcileOrdersCommand(service usecase.IReconcileOrders) *cli.Command {
	return &cli.Command{
		Name:   "reconcile-orders",
		Usage:  "recover order intents after API or network failures",
		Action: func(*cli.Context) error { return service.Process(context.Background()) },
	}
}
