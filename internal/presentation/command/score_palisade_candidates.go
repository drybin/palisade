package command

import (
	"context"

	"github.com/drybin/palisade/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewScorePalisadeCandidatesCommand(service usecase.IScorePalisadeCandidates) *cli.Command {
	return &cli.Command{
		Name: "score-palisade-candidates", Usage: "send dry-run palisade signals to Telegram",
		Flags:  []cli.Flag{&cli.BoolFlag{Name: "debug"}},
		Action: func(c *cli.Context) error { return service.Process(context.Background(), c.Bool("debug")) },
	}
}
