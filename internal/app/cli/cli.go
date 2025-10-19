package cli

import (
	"log"
	"os"

	"github.com/drybin/palisade/internal/app/cli/config"
	"github.com/drybin/palisade/internal/app/cli/registry"
	"github.com/drybin/palisade/internal/presentation/command"
	"github.com/joho/godotenv"
	cliV2 "github.com/urfave/cli/v2"
)

const cliAppDesc = "cli tool for fead-and-greed"

// example call go run --race ./cmd/cli/... hello-world
func Run(config *config.Config) error {
	if err := godotenv.Load(); err != nil {
		log.Println(err)
	}

	cnt, err := registry.NewContainer(config)
	if err != nil {
		log.Fatal("failed to create cli container", err)
	}

	app := cliV2.NewApp()
	app.Name = config.ServiceName
	app.Usage = cliAppDesc
	app.Commands = []*cliV2.Command{
		command.NewHelloWorldCommand(cnt.Usecases.HelloWorld),
		command.NewPalisadeProcessCommand(cnt.Usecases.PalisadeProcess),
		command.NewGetCoinListCommand(cnt.Usecases.GetCoinList),
	}

	return app.Run(os.Args)
}
