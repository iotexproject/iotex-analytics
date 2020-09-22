// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: go build -o ./bin/server -v .
// ./bin/server

package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-analytics/services/mimo"
	"github.com/iotexproject/iotex-analytics/sql"
)

const defaultPort = "8089"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	chainEndpoint := os.Getenv("CHAIN_ENDPOINT")
	if chainEndpoint == "" {
		chainEndpoint = "api.testnet.iotex.one:80"
	}

	mimoFactoryAddrStr := os.Getenv("MIMO_FACTORY_ADDRESS")
	if mimoFactoryAddrStr == "" {
		mimoFactoryAddrStr = "io1vu0tq2v6ph5xhwrrpx0vzvg5wt8adfk3ygnxfj"
	}
	mimoFactoryAddr, err := address.FromString(mimoFactoryAddrStr)
	if err != nil {
		log.L().Panic("failed to parse mimo factory address", zap.Error(err))
	}
	mimoFactoryCreationHeightStr := os.Getenv("MIMO_FACTORY_CREATION_HEIGHT")
	if mimoFactoryCreationHeightStr == "" {
		mimoFactoryCreationHeightStr = "5383913"
	}
	mimoFactoryCreationHeight, err := strconv.ParseUint(mimoFactoryCreationHeightStr, 10, 64)
	if err != nil {
		log.L().Panic("failed to parse mimo factory creation height", zap.Error(err))
	}
	connectionStr := os.Getenv("CONNECTION_STRING")
	if connectionStr == "" {
		log.L().Panic("failed to get db connection config")
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "mimo_testnet"
	}
	grpcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(grpcCtx, chainEndpoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to chain's API server.")
	}
	service := mimo.NewService(iotexapi.NewAPIServiceClient(conn), sql.NewMySQL(connectionStr, dbName), mimoFactoryAddr, mimoFactoryCreationHeight, nil, 100)

	http.Handle("/", graphqlHandler(handler.Playground("GraphQL playground", "/query")))
	http.Handle("/query", graphqlHandler(handler.GraphQL(service.ExecutableSchema())))
	http.Handle("/metrics", promhttp.Handler())
	log.S().Infof("connect to http://localhost:%s/ for GraphQL playground", port)

	// Start GraphQL query service
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.L().Fatal("Failed to serve index query service", zap.Error(err))
		}
	}()

	if err := service.Start(context.Background()); err != nil {
		log.L().Fatal("Failed to start the indexer", zap.Error(err))
	}

	defer func() {
		if err := service.Stop(context.Background()); err != nil {
			log.L().Fatal("Failed to stop the indexer", zap.Error(err))
		}
	}()

	select {}
}

func graphqlHandler(playgroundHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		playgroundHandler.ServeHTTP(w, r)
	})
}
