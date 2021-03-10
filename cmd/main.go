package main

import (
	"os"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-analytics/cmd/server"
)

var (
	version = "0.1.0-dev"
)

func main() {
	app := cli.NewApp()
	app.Name = "iotex-analytics"
	app.Usage = "iotex-analytics is Analytics Platform for Iotex Smart Chain"
	app.Version = version
	app.Commands = []*cli.Command{
		server.Commands,
	}
	if err := app.Run(os.Args); err != nil {
		log.L().Fatal("Failed to start application", zap.Error(err))
	}
}
