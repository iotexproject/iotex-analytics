package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-analytics/graphql"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/sql"
)

const defaultPort = "8089"

func main() {
	var grpcAddr string

	flag.StringVar(&grpcAddr, "grpc-addr", "127.0.0.1:14014", "grpc address for chain's API service")
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	//ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.NumDelegates = 3
	cfg.Genesis.NumSubEpochs = 2
	cfg.Genesis.NumCandidateDelegates = 3

	store := sql.NewSQLite3(cfg.DB.SQLITE3)

	idx := indexservice.NewIndexer(store)
	if err := idx.RegisterDefaultProtocols(cfg); err != nil {
		log.L().Fatal("Failed to register default protocols")
	}

	grpcctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(grpcctx, grpcAddr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to API server.")
	}
	client := iotexapi.NewAPIServiceClient(conn)

	ctx := indexservice.WithIndexerCtx(context.Background(), indexservice.IndexerCtx{
		Client: client,
	})

	if err := idx.Start(ctx); err != nil {
		log.L().Fatal("Failed to start the indexer", zap.Error(err))
	}

	defer func() {
		if err := idx.Stop(ctx); err != nil {
			log.L().Fatal("Failed to stop the indexer", zap.Error(err))
		}
	}()

	http.Handle("/", handler.Playground("GraphQL playground", "/query"))
	http.Handle("/query", handler.GraphQL(graphql.NewExecutableSchema(graphql.Config{Resolvers: &graphql.Resolver{idx}})))

	log.S().Infof("connect to http://localhost:%s/ for GraphQL playground", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.L().Fatal("Failed to serve indexing service", zap.Error(err))
	}
}
