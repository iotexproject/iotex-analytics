package indexservice

import (
	"context"
	"math/big"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-analytics/indexer"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (

	// IndexService is the main indexer service, which coordinates the building of all indexers
	IndexService struct {
		dao          blockdao.BlockDAO
		indexers     []indexer.AsyncIndexer
		mutex        sync.RWMutex
		terminate    chan bool
		wg           sync.WaitGroup
		subscribers  []chan uint64
		indexerLocks []bool

		chainClient iotexapi.APIServiceClient
		batchSize   uint64
	}
)

// NewIndexService creates a new indexer service
func NewIndexService(chainClient iotexapi.APIServiceClient, batchSize uint64, bc blockdao.BlockDAO, indexers []indexer.AsyncIndexer) *IndexService {
	return &IndexService{
		dao:          bc,
		indexers:     indexers,
		subscribers:  make([]chan uint64, len(indexers)),
		terminate:    make(chan bool),
		wg:           sync.WaitGroup{},
		indexerLocks: make([]bool, len(indexers)),

		chainClient: chainClient,
		batchSize:   batchSize,
	}
}

func (is *IndexService) startIndex(ctx context.Context, i int) error {
	if err := is.indexers[i].Start(ctx); err != nil {
		return err
	}
	go func() {
		is.wg.Add(1)
		for {
			select {
			case <-is.terminate:
				is.wg.Done()
				return
			case tipHeight := <-is.subscribers[i]:
				{
					is.mutex.Lock()
					defer is.mutex.Unlock()
					if is.indexerLocks[i] {
						continue
					}
					is.indexerLocks[i] = true
				}
				defer func() {
					is.mutex.Lock()
					is.indexerLocks[i] = false
					is.mutex.Unlock()
				}()
				nextHeight, err := is.indexers[i].NextHeight(ctx)
				if err != nil {
					log.S().Panic("failed to get next height", zap.Error(err))
				}
				for nextHeight < tipHeight {
					blk, err := is.dao.GetBlockByHeight(nextHeight)
					if err != nil {
						log.S().Panicf("failed to read block from dao %v\n", err)
					}
					receipts, err := is.dao.GetReceipts(nextHeight)
					if err != nil {
						log.S().Panicf("failed to read receipts from dao %v\n", err)
					}
					blk.Receipts = receipts
					actionReceipts := make(map[hash.Hash256]*action.Receipt, len(receipts))
					for _, receipt := range receipts {
						actionReceipts[receipt.ActionHash] = receipt
					}
					tlogs, err := is.dao.TransactionLogs(nextHeight)
					if err != nil {
						log.S().Panicf("failed to read transaction logs from dao %v\n", err)
					}
					for _, l := range tlogs.Logs {
						logs := make([]*action.TransactionLog, len(l.Transactions))
						for i, txn := range l.Transactions {
							amount, ok := new(big.Int).SetString(txn.Amount, 10)
							if !ok {
								log.S().Panicf("failed to parse %s\n", txn.Amount)
							}
							logs[i] = &action.TransactionLog{
								Type:      txn.Type,
								Amount:    amount,
								Sender:    txn.Sender,
								Recipient: txn.Recipient,
							}
						}
						actionReceipts[hash.BytesToHash256(l.ActionHash)].AddTransactionLogs(logs...)
					}
					if err := is.indexers[i].PutBlock(ctx, blk); err != nil {
						log.S().Panicf("failed to put data to indexer %v\n", err)
					}
					nextHeight++
				}
			}
		}
	}()
	return nil
}

// Start starts the index service
func (is *IndexService) Start(ctx context.Context) error {
	if err := is.dao.Start(ctx); err != nil {
		return err
	}

	for i := 0; i < len(is.indexers); i++ {
		if err := is.startIndex(ctx, i); err != nil {
			return err
		}
	}
	go func() {
		for {
			select {
			case <-is.terminate:
				return
			default:
				res, err := is.chainClient.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
				if err != nil {
					log.L().Error("failed to get chain meta", zap.Error(err))
				} else {
					if err := is.fetchAndBuild(ctx, res.ChainMeta.Height); err != nil {
						log.L().Error("failed to index blocks", zap.Error(err))
					}
				}
			}
		}
	}()

	return nil
}

// Stop stops the index service
func (is *IndexService) Stop(ctx context.Context) error {
	close(is.terminate)
	is.wg.Wait()
	return is.dao.Stop(ctx)
}

func (is *IndexService) fetchAndBuild(ctx context.Context, tipHeight uint64) error {
	lastHeight, err := is.dao.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height from block dao")
	}
	log.L().Debug("fetch dao height", zap.Uint64("height", lastHeight))
	chainClient := is.chainClient
	for startHeight := lastHeight + 1; startHeight <= tipHeight; {
		count := is.batchSize
		if count > tipHeight-startHeight+1 {
			count = tipHeight - startHeight + 1
		}
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
			receipts := map[hash.Hash256]*action.Receipt{}
			for _, receiptPb := range blkInfo.GetReceipts() {
				receipt := &action.Receipt{}
				receipt.ConvertFromReceiptPb(receiptPb)
				receipts[receipt.ActionHash] = receipt
				blk.Receipts = append(blk.Receipts, receipt)
			}
			transactionLogs, err := chainClient.GetTransactionLogByBlockHeight(
				context.Background(),
				&iotexapi.GetTransactionLogByBlockHeightRequest{
					BlockHeight: blk.Header.Height(),
				},
			)
			if err != nil {
				return errors.Wrap(err, "failed to fetch transaction logs")
			}
			for _, tlogs := range transactionLogs.TransactionLogs.Logs {
				logs := make([]*action.TransactionLog, len(tlogs.Transactions))
				for i, txn := range tlogs.Transactions {
					amount, ok := new(big.Int).SetString(txn.Amount, 10)
					if !ok {
						return errors.Errorf("failed to parse %s", txn.Amount)
					}
					logs[i] = &action.TransactionLog{
						Type:      txn.Type,
						Amount:    amount,
						Sender:    txn.Sender,
						Recipient: txn.Recipient,
					}
				}
				actHash := hash.BytesToHash256(tlogs.ActionHash)
				receipts[actHash].AddTransactionLogs(logs...)
			}
			if err := is.dao.PutBlock(ctx, blk); err != nil {
				return errors.Wrap(err, "failed to build index for the block")
			}
			blkHeight := blk.Height()
			for i := 0; i < len(is.subscribers); i++ {
				is.subscribers[i] <- blkHeight
			}
		}
		startHeight += count
	}
	return nil
}
