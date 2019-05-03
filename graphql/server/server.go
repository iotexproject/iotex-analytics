// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: go build -o ./bin/server -v ./graphql/server
// ./bin/server

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
	"io/ioutil"

	"github.com/iotexproject/iotex-analytics/graphql"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/sql"
)

const defaultPort = "8089"

func main() {
	var grpcAddr string
	var configPath string

	flag.StringVar(&grpcAddr, "grpc-addr", "127.0.0.1:14014", "grpc address for chain's API service")
	flag.StringVar(&configPath, "config", "config.yaml", "path of server config file")
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.L().Fatal("Failed to load config file", zap.Error(err))
	}
	var cfg indexservice.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.L().Fatal("failed to unmarshal config", zap.Error(err))
	}

	store := sql.NewSQLite3(cfg.DBPath)

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
