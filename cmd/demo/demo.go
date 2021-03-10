package demo

import (
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/urfave/cli/v2"
)

var Demo = &cli.Command{
	Name:        "demo",
	Usage:       "demo",
	Description: `demo`,
	Action:      runDemo,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "config.yaml",
			Usage:   "Load configuration from `FILE`",
		},
	},
}

func runDemo(c *cli.Context) error {

	if c.String("config") == "" {
		log.L().Fatal("Cannot determine default configuration path.")
	}
	log.L().Info("demo test")

	return nil
}
