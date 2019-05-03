package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/handler"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-analytics/graphql"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/protocol"
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

	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.NumDelegates = 3
	cfg.Genesis.NumSubEpochs = 2
	cfg.Genesis.NumCandidateDelegates = 3

	store := sql.NewSQLite3(cfg.DB.SQLITE3)
	if err := store.Start(ctx); err != nil {
		log.L().Fatal("Failed to start the underlying db")
	}
	defer func() {
		if err := store.Stop(ctx); err != nil {
			log.L().Fatal("Failed to stop the underlying db")
		}
	}()

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

	if err := startIndexer(idx, client); err != nil {
		log.L().Fatal("Failed to start the indexer", zap.Error(err))
	}

	http.Handle("/", handler.Playground("GraphQL playground", "/query"))
	http.Handle("/query", handler.GraphQL(graphql.NewExecutableSchema(graphql.Config{Resolvers: &graphql.Resolver{idx}})))

	log.S().Infof("connect to http://localhost:%s/ for GraphQL playground", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.L().Fatal("Failed to serve indexing service", zap.Error(err))
	}
}

func startIndexer(idx *indexservice.Indexer, client iotexapi.APIServiceClient) error {
	ctx := context.Background()

	lastHeight, err := idx.GetLastHeight()
	//switch {
	//case errors.Cause(err) == protocol.ErrNotExist:
	//case errors.Cause(err) != nil:
	//	return errors.Wrap(err, "error when getting last inserted height from underlying db")
	//}
	if err != nil {
		if err := idx.CreateTablesIfNotExist(); err != nil {
			return errors.Wrap(err, "failed to create tables")
		}

		readStateRequest := &iotexapi.ReadStateRequest{
			ProtocolID: []byte(poll.ProtocolID),
			MethodName: []byte("DelegatesByEpoch"),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(uint64(1))},
		}
		res, err := client.ReadState(ctx, readStateRequest)
		if err != nil {
			return errors.Wrap(err, "failed to read genesis delegates from blockchain")
		}
		var genesisDelegates state.CandidateList
		if err := genesisDelegates.Deserialize(res.Data); err != nil {
			return errors.Wrap(err, "failed to deserialize gensisDelegates")
		}
		gensisConfig := &protocol.GenesisConfig{InitCandidates: genesisDelegates}

		// Initialize indexer
		if err := idx.Initialize(gensisConfig); err != nil {
			return errors.Wrap(err, "failed to initialize the indexer")
		}
	}

	getChainMetaRes, err := client.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to get chain metadata")
	}
	tipHeight := getChainMetaRes.ChainMeta.Height

	getRawBlocksRes, err := client.GetRawBlocks(ctx, &iotexapi.GetRawBlocksRequest{
		StartHeight: lastHeight + 1,
		Count:       tipHeight - lastHeight,
	})
	for _, blkPb := range getRawBlocksRes.Blocks {
		blk := &block.Block{}
		if err := blk.ConvertFromBlockPb(blkPb); err != nil {
			return errors.Wrap(err, "failed to convert block protobuf to raw block")
		}
		if err := idx.BuildIndex(blk); err != nil {
			return errors.Wrap(err, "failed to build index the block")
		}
	}
	// TODO: Add polling mechanism for newly committed blocks
	return nil
}
