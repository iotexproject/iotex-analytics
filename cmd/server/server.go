package server

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/iotexproject/iotex-analytics/config"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var Commands = &cli.Command{
	Name:        "server",
	Usage:       "start server",
	Description: `start server`,
	Action:      runServer,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   config.FindDefaultConfigPath(),
			Usage:   "Load configuration from `FILE`",
		},
	},
}

func runServer(c *cli.Context) error {
	var err error
	var cfg *config.Config
	if c.String("config") == "" {
		log.L().Fatal("Cannot determine default configuration path.",
			zap.Any("DefaultConfigFiles", config.DefaultConfigFiles),
			zap.Any("DefaultConfigDirs", config.DefaultConfigDirs))
	}

	cfg, err = config.New(c.String("config"))
	if err != nil {
		return err
	}

	store := sql.CreateMySQLStore(
		cfg.Database.UserName,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)

	ctx := sql.WithStore(c.Context, store)

	grpcCtx1, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn1, err := grpc.DialContext(grpcCtx1, cfg.Iotex.ChainEndPoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to chain's API server.")
	}
	chainClient := iotexapi.NewAPIServiceClient(conn1)
	ids := indexservice.NewIndexService(chainClient, 1, nil, nil)

	if err := ids.Start(ctx); err != nil {
		log.L().Fatal("Failed to start the indexer", zap.Error(err))
	}

	handleShutdown(ids)

	return nil
}

type Stopper interface {
	Stop(context.Context) error
}

func handleShutdown(service ...Stopper) {
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, os.Kill)

	// wait INT or KILL
	<-stop
	log.L().Info("shutting down ...")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, s := range service {
		if err := s.Stop(ctx); err != nil {
			log.L().Error("shutdown", zap.Error(err))
		}
	}
	return
}
