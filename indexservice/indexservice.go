package indexservice

import (
	"context"
	"math/big"
	"sync"
	"sync/atomic"

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

type AtomicBool struct{ flag int32 }

func (b *AtomicBool) Set(value bool) {
	var i int32 = 0
	if value {
		i = 1
	}
	atomic.StoreInt32(&(b.flag), int32(i))
}

func (b *AtomicBool) Get() bool {
	return atomic.LoadInt32(&(b.flag)) != 0
}

type (

	// IndexService is the main indexer service, which coordinates the building of all indexers
	IndexService struct {
		dao          blockdao.BlockDAO
		indexers     []indexer.AsyncIndexer
		terminate    chan bool
		wg           sync.WaitGroup
		subscribers  []chan uint64
		indexerLocks []AtomicBool

		chainClient iotexapi.APIServiceClient
		batchSize   uint64
	}
)

// NewIndexService creates a new indexer service
func NewIndexService(chainClient iotexapi.APIServiceClient, batchSize uint64, bc blockdao.BlockDAO, indexers []indexer.AsyncIndexer) *IndexService {
	subscribers := make([]chan uint64, len(indexers))
	for i := range indexers {
		subscribers[i] = make(chan uint64, 0)
	}
	return &IndexService{
		dao:          bc,
		indexers:     indexers,
		subscribers:  subscribers,
		terminate:    make(chan bool),
		wg:           sync.WaitGroup{},
		indexerLocks: make([]AtomicBool, len(indexers)),

		chainClient: chainClient,
		batchSize:   batchSize,
	}
}

func (is *IndexService) startIndex(ctx context.Context, i int) error {
	if err := is.indexers[i].Start(ctx); err != nil {
		return err
	}
	is.wg.Add(1)
	go func() {
		defer is.wg.Done()
		for {
			select {
			case <-is.terminate:
				return
			case tipHeight := <-is.subscribers[i]:

				if is.indexerLocks[i].Get() {
					continue
				}
				is.indexerLocks[i].Set(true)

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
				is.indexerLocks[i].Set(false)
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
	is.wg.Wait()

	return nil
}

// Stop stops the index service
func (is *IndexService) Stop(ctx context.Context) error {
	close(is.terminate)
	for i := 0; i < len(is.indexers); i++ {
		if err := is.indexers[i].Stop(ctx); err != nil {
			return err
		}
	}
	return is.dao.Stop(ctx)
}

func (is *IndexService) fetchAndBuild(ctx context.Context, tipHeight uint64) error {
	lastHeight, err := is.dao.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height from block dao")
	}
	log.L().Debug("fetch height", zap.Uint64("daoHeight", lastHeight), zap.Uint64("tipHeight", tipHeight))
	chainClient := is.chainClient
	for startHeight := lastHeight + 1; startHeight <= tipHeight; {
		count := is.batchSize
		if count > tipHeight-startHeight+1 {
			count = tipHeight - startHeight + 1
		}
		rawRequest := &iotexapi.GetRawBlocksRequest{
			StartHeight:  startHeight,
			Count:        count,
			WithReceipts: true,
		}

		getRawBlocksRes, err := chainClient.GetRawBlocks(context.Background(), rawRequest)
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
			for _, subscriber := range is.subscribers {
				subscriber <- blkHeight
			}
		}
		startHeight += count
	}
	return nil
}
