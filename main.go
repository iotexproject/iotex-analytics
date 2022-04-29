// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// usage: go build -o ./bin/server -v .
// ./bin/server

package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-analytics/graphql"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol/actions"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta"
	"github.com/iotexproject/iotex-analytics/queryprotocol/hermes2"
	"github.com/iotexproject/iotex-analytics/queryprotocol/productivity"
	"github.com/iotexproject/iotex-analytics/queryprotocol/rewards"
	"github.com/iotexproject/iotex-analytics/queryprotocol/votings"
	"github.com/iotexproject/iotex-analytics/sql"
)

const defaultPort = "8089"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	chainEndpoint := os.Getenv("CHAIN_ENDPOINT")
	if chainEndpoint == "" {
		chainEndpoint = "127.0.0.1:14014"
	}

	electionEndpoint := os.Getenv("ELECTION_ENDPOINT")
	if electionEndpoint == "" {
		electionEndpoint = "127.0.0.1:8090"
	}

	connectionStr := os.Getenv("CONNECTION_STRING")
	if connectionStr == "" {
		connectionStr = "root:rootuser@tcp(127.0.0.1:3306)/"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "analytics"
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.L().Fatal("Failed to load config file", zap.Error(err))
	}
	var cfg indexservice.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.L().Fatal("failed to unmarshal config", zap.Error(err))
	}

	if cfg.Zap == nil {
		zapCfg := zap.NewProductionConfig()
		cfg.Zap = &zapCfg
	} else {
		if cfg.Zap.Development {
			cfg.Zap.EncoderConfig = zap.NewDevelopmentEncoderConfig()
		} else {
			cfg.Zap.EncoderConfig = zap.NewProductionEncoderConfig()
		}
	}
	cfg.Zap.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Zap.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	logger, err := cfg.Zap.Build()
	if err == nil {
		zap.ReplaceGlobals(logger)
	}
	readOnly := os.Getenv("READ_ONLY")
	if readOnly != "" {
		cfg.ReadOnly = readOnly == "true"
	}

	store := sql.NewMySQL(connectionStr, dbName, cfg.ReadOnly)
	maxOpenConnsStr := os.Getenv("MAX_OPEN_CONNECTIONS")
	if maxOpenConnsStr != "" {
		maxOpenConns, err := strconv.Atoi(maxOpenConnsStr)
		if err != nil {
			log.L().Info("failed to parse parameter", zap.String("MAX_OPEN_CONNECTIONS", maxOpenConnsStr), zap.Error(err))
		}
		store.SetMaxOpenConns(maxOpenConns)
	}

	idx := indexservice.NewIndexer(store, cfg)
	if err := idx.RegisterDefaultProtocols(); err != nil {
		log.L().Fatal("Failed to register default protocols", zap.Error(err))
	}

	http.Handle("/", graphqlHandler(handler.Playground("GraphQL playground", "/query")))
	http.Handle("/query", graphqlHandler(handler.GraphQL(graphql.NewExecutableSchema(graphql.Config{Resolvers: &graphql.Resolver{
		PP: productivity.NewProtocol(idx),
		RP: rewards.NewProtocol(idx),
		VP: votings.NewProtocol(idx),
		AP: actions.NewProtocol(idx),
		CP: chainmeta.NewProtocol(idx),
		HP: hermes2.NewProtocol(idx, cfg.HermesConfig),
	}}))))
	//http.Handle("/metrics", promhttp.Handler())
	//log.S().Infof("connect to http://localhost:%s/ for GraphQL playground", port)

	// Start GraphQL query service
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.L().Fatal("Failed to serve index query service", zap.Error(err))
		}
	}()

	grpcCtx1, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn1, err := grpc.DialContext(grpcCtx1, chainEndpoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to chain's API server.")
	}
	chainClient := iotexapi.NewAPIServiceClient(conn1)

	grpcCtx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn2, err := grpc.DialContext(grpcCtx2, electionEndpoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to election's API server.")
	}
	electionClient := api.NewAPIServiceClient(conn2)

	ctx := indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: idx.Config.ConsensusScheme,
	})

	if err := idx.Start(ctx); err != nil {
		log.L().Fatal("Failed to start the indexer", zap.Error(err))
	}

	defer func() {
		if err := idx.Stop(ctx); err != nil {
			log.L().Fatal("Failed to stop the indexer", zap.Error(err))
		}
	}()

	select {}
}

func graphqlHandler(playgroundHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "POST" {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.L().Error("Failed to read request body", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r1 := r.Clone(r.Context())
			// clone body
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
			r1.Body = ioutil.NopCloser(bytes.NewReader(body))
			statsHandler(r1.Context(), r1)
		}
		playgroundHandler.ServeHTTP(w, r)
	})
}

func statsHandler(ctx context.Context, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	clientID := r.Header.Get("x-iotex-client-id")
	if clientID == "" {
		clientID = "unknown"
	}
	var body []byte
	if r.Body != nil {
		body, _ = ioutil.ReadAll(r.Body)
	}
	log.L().Info("request stat", zap.String("clientIP", clientIP), zap.String("clientID", clientID), zap.ByteString("body", body))
}
