package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewPalisadeProcessMultiCommand(service *usecase.PalisadeProcessMulti) *cli.Command {
	return &cli.Command{
		Name:  "process_multi",
		Usage: "process multi command - processes multiple palisade coins",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return service.Process(context.Background())
		},
	}
}

