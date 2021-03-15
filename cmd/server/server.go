package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iotexproject/iotex-analytics/config"
	"github.com/iotexproject/iotex-analytics/indexer"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
	if err := log.InitLoggers(cfg.Log, cfg.SubLogs); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	store := sql.CreateMySQLStore(
		cfg.Mysql.UserName,
		cfg.Mysql.Password,
		cfg.Mysql.Host,
		cfg.Mysql.Port,
		cfg.Mysql.DBName,
	)
	if err := store.Start(c.Context); err != nil {
		log.L().Fatal("Failed to start mysql store", zap.Error(err))
	}
	ctx := sql.WithStore(c.Context, store)

	grpcCtx1, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn1, err := grpc.DialContext(grpcCtx1, cfg.Iotex.ChainEndPoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to chain's API server.")
	}
	chainClient := iotexapi.NewAPIServiceClient(conn1)

	var indexers []blockdao.BlockIndexer
	var dao blockdao.BlockDAO
	if false {
		dao = blockdao.NewBlockDAOInMemForTest(indexers)
	} else {
		dao = blockdao.NewBlockDAO(indexers, cfg.BlockDB)
	}
	var tip protocol.TipInfo
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis: genesis.Default,
			Tip:     tip,
		},
	)
	var asyncindexers []indexer.AsyncIndexer

	asyncindexers = append(asyncindexers, indexer.NewBlockIndexer())
	//asyncindexers = append(asyncindexers, indexer.NewBlockMetaIndexer(store))
	ids := indexservice.NewIndexService(chainClient, 64, dao, asyncindexers)
	go func() {
		if err := ids.Start(ctx); err != nil {
			log.L().Fatal("Failed to start the indexer", zap.Error(err))
		}
	}()
	handleShutdown(ctx, ids)

	return nil
}

type Stopper interface {
	Stop(context.Context) error
}

func handleShutdown(ctx context.Context, service ...Stopper) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// wait INT or KILL
	<-stop
	log.L().Info("shutting down ...")
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for _, s := range service {
		if err := s.Stop(ctx); err != nil {
			log.L().Error("shutdown", zap.Error(err))
		}
	}
	return
}
