// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mimo

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accountbalance"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blockinfo"
	mimoprotocol "github.com/iotexproject/iotex-analytics/indexprotocol/mimo"
	"github.com/iotexproject/iotex-analytics/indexprotocol/xrc20"
	"github.com/iotexproject/iotex-analytics/services"
	s "github.com/iotexproject/iotex-analytics/sql"
)

var (
	blockHeightMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "block_height",
			Help: "block height",
		},
		[]string{},
	)
)

// mimoService defines a service for mimo
type mimoService struct {
	store                 s.Store
	protocols             []indexprotocol.ProtocolV2
	chainClient           iotexapi.APIServiceClient
	lastHeight            uint64
	batchSize             uint64
	terminate             chan bool
	factoryCreationHeight *big.Int

	exchanges map[string]bool
}

// NewService creates a new mimo service
func NewService(chainClient iotexapi.APIServiceClient, store s.Store, mimoFactoryAddr address.Address, factoryCreationHeight uint64, initBalances map[string]*big.Int, batchSize uint8) services.Service {
	return &mimoService{
		chainClient:           chainClient,
		store:                 store,
		factoryCreationHeight: new(big.Int).SetUint64(factoryCreationHeight),
		protocols: []indexprotocol.ProtocolV2{
			blockinfo.NewProtocol(),
			mimoprotocol.NewProtocol(mimoFactoryAddr),
			accountbalance.NewProtocolWithMonitorTable(mimoprotocol.ExchangeMonitorViewName, initBalances),
			xrc20.NewProtocolWithMonitorTokenAndAddresses(mimoprotocol.TokenMonitorViewName),
		},
		batchSize: uint64(batchSize),
	}
}

// Start starts the service
func (service *mimoService) Start(ctx context.Context) error {
	prometheus.MustRegister(blockHeightMtc)
	if err := service.store.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start db")
	}
	if err := service.store.Transact(func(tx *sql.Tx) error {
		for _, p := range service.protocols {
			if err := p.Initialize(ctx, tx); err != nil {
				return errors.Wrap(err, "failed to initialize the protocol")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	lastHeight, err := service.latestHeight()
	if err != nil && err != indexprotocol.ErrNotExist {
		return errors.Wrap(err, "failed to get current epoch and tip height")
	}
	service.lastHeight = lastHeight.Uint64()

	log.L().Info("Catching up via network")
	chainMetaResponse, err := service.chainClient.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to get chain metadata")
	}
	tipHeight := chainMetaResponse.GetChainMeta().GetHeight()
	if err := service.indexInBatch(ctx, tipHeight); err != nil {
		return errors.Wrap(err, "failed to index blocks in batch")
	}

	log.L().Info("Subscribing to new blocks")
	heightChan := make(chan uint64)
	reportChan := make(chan error)
	go func() {
		for {
			select {
			case <-service.terminate:
				service.terminate <- true
				return
			case tipHeight := <-heightChan:
				// index blocks up to this height
				if err := service.indexInBatch(ctx, tipHeight); err != nil {
					log.L().Error("failed to index blocks in batch", zap.Error(err))
				}
			case err := <-reportChan:
				log.L().Error("something goes wrong", zap.Error(err))
			}
		}
	}()
	service.subscribeNewBlock(service.chainClient, heightChan, reportChan, service.terminate)
	return nil
}

// Stop stops the service
func (service *mimoService) Stop(ctx context.Context) error {
	service.terminate <- true
	return service.store.Stop(ctx)
}

func (service *mimoService) indexInBatch(ctx context.Context, tipHeight uint64) error {
	chainClient := service.chainClient

	startHeight := service.lastHeight + 1
	for startHeight <= tipHeight {
		count := service.batchSize
		if service.batchSize > tipHeight-startHeight+1 {
			count = tipHeight - startHeight + 1
		}
		fmt.Printf("%d -> %d\n", startHeight, startHeight+count)
		getRawBlocksRes, err := chainClient.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
			StartHeight:  startHeight,
			Count:        count,
			WithReceipts: true,
		})
		if err != nil {
			return errors.Wrap(err, "failed to get raw blocks from the chain")
		}
		for _, blkInfo := range getRawBlocksRes.GetBlocks() {
			blk := &block.Block{}
			if err := blk.ConvertFromBlockPb(blkInfo.GetBlock()); err != nil {
				return errors.Wrap(err, "failed to convert block protobuf to raw block")
			}

			for _, receiptPb := range blkInfo.GetReceipts() {
				receipt := &action.Receipt{}
				receipt.ConvertFromReceiptPb(receiptPb)
				blk.Receipts = append(blk.Receipts, receipt)
			}
			var transactionLogs []*iotextypes.TransactionLog
			// TODO: add transaction log via GetRawBlocks
			if blk.Height() >= 5383954 {
				transactionLogResponse, err := chainClient.GetTransactionLogByBlockHeight(context.Background(), &iotexapi.GetTransactionLogByBlockHeightRequest{
					BlockHeight: blk.Height(),
				})
				if err != nil {
					return errors.Wrapf(err, "failed to fetch transaction log of block %d", blk.Height())
				}
				transactionLogs = transactionLogResponse.GetTransactionLogs().GetLogs()
			}
			if err := service.buildIndex(ctx, &indexprotocol.BlockData{
				Block:           blk,
				TransactionLogs: transactionLogs,
			}); err != nil {
				return errors.Wrap(err, "failed to build index for the block")
			}
			// Update lastHeight tracker
			service.lastHeight = blk.Height()
			blockHeightMtc.With(prometheus.Labels{}).Set(float64(service.lastHeight))
		}
		startHeight += count
	}
	return nil
}

func (service *mimoService) subscribeNewBlock(
	client iotexapi.APIServiceClient,
	height chan uint64,
	report chan error,
	unsubscribe chan bool,
) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-unsubscribe:
				unsubscribe <- true
				return
			case <-ticker.C:
				if res, err := client.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{}); err != nil {
					report <- err
				} else {
					height <- res.ChainMeta.Height
				}
			}
		}
	}()
}

func (service *mimoService) buildIndex(ctx context.Context, data *indexprotocol.BlockData) error {
	return service.store.Transact(func(tx *sql.Tx) error {
		for _, p := range service.protocols {
			if err := p.HandleBlockData(ctx, tx, data); err != nil {
				return errors.Wrapf(err, "failed to build index for block on height %d", data.Block.Height())
			}
		}
		return nil
	})
}

func (service *mimoService) latestHeight() (*big.Int, error) {
	var n *big.Int
	var s sql.NullString
	err := service.store.GetDB().QueryRow("SELECT coalesce(MAX(block_height)) FROM `" + blockinfo.TableName + "`").Scan(&s)
	if err != nil {
		return nil, err
	}
	if s.Valid {
		n, _ = new(big.Int).SetString(s.String, 10)
		return n, nil
	}
	return service.factoryCreationHeight, nil
}
