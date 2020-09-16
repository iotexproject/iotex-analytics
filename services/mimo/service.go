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
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
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

type (
	mimoService struct {
		store                 s.Store
		protocols             []indexprotocol.ProtocolV2
		chainClient           iotexapi.APIServiceClient
		lastHeight            uint64
		batchSize             uint64
		terminate             chan bool
		factoryCreationHeight *big.Int
	}
	// AddressPair defines a struct storing a pair of addresses
	AddressPair struct {
		Address1 string
		Address2 string
	}
)

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

func (service *mimoService) Stop(ctx context.Context) error {
	service.terminate <- true
	return service.store.Stop(ctx)
}

func (service *mimoService) ExecutableSchema() graphql.ExecutableSchema {
	return NewExecutableSchema(Config{Resolvers: service})
}

func (service *mimoService) Query() QueryResolver {
	return &queryResolver{service}
}

func (service *mimoService) indexInBatch(ctx context.Context, tipHeight uint64) error {
	chainClient := service.chainClient
	ctx = services.WithServiceClient(ctx, chainClient)
	startHeight := service.lastHeight + 1
	for startHeight <= tipHeight {
		count := service.batchSize
		if service.batchSize > tipHeight-startHeight+1 {
			count = tipHeight - startHeight + 1
		}
		fmt.Printf("indexing blocks %d -> %d\n", startHeight, startHeight+count)
		getRawBlocksRes, err := chainClient.GetRawBlocks(ctx, &iotexapi.GetRawBlocksRequest{
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
				transactionLogResponse, err := chainClient.GetTransactionLogByBlockHeight(ctx, &iotexapi.GetTransactionLogByBlockHeightRequest{
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
	if err := service.store.GetDB().QueryRow("SELECT coalesce(MAX(block_height)) FROM `" + blockinfo.TableName + "`").Scan(&s); err != nil {
		return nil, err
	}
	if s.Valid {
		n, _ = new(big.Int).SetString(s.String, 10)
		return n, nil
	}
	return service.factoryCreationHeight, nil
}

func (service *mimoService) exchanges(height uint64, offset uint32, count uint8) ([]AddressPair, error) {
	rows, err := service.store.GetDB().Query(
		"SELECT `exchange`, `token` FROM `"+mimoprotocol.ExchangeCreationTableName+"` WHERE `block_height` <= ? AND `id` > ? ORDER BY `id` LIMIT ?",
		height,
		offset,
		count,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query exchanges")
	}
	pairs := []AddressPair{}
	for rows.Next() {
		var exchange, token string
		if err := rows.Scan(&exchange, &token); err != nil {
			return nil, errors.Wrap(err, "failed to parse exchange record")
		}
		pairs = append(pairs, AddressPair{
			Address1: exchange,
			Address2: token,
		})
	}
	return pairs, nil
}

func (service *mimoService) totalVolumes(days uint8) (map[string]*big.Int, error) {
	if days == 0 {
		days = 1
	}
	rows, err := service.store.GetDB().Query(
		"SELECT DATE(bi.timestamp) d, SUM(t.amount) v "+
			"FROM `"+accountbalance.TransactionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"INNER JOIN `"+mimoprotocol.ExchangeMonitorViewName+"` e "+
			"ON e.account = t.sender OR e.account = t.recipient "+
			"WHERE bi.timestamp >= ? "+
			"GROUP BY d "+
			"ORDER BY d",
		time.Now().UTC().Add(-time.Duration((days-1)*24)*time.Hour).Truncate(24*time.Hour).String(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query volumes")
	}
	ret := map[string]*big.Int{}
	for rows.Next() {
		var date time.Time
		var volumeStr string
		if err := rows.Scan(&date, &volumeStr); err != nil {
			return nil, errors.Wrap(err, "failed to parse volume information")
		}
		volume, ok := new(big.Int).SetString(volumeStr, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse volume %s", volumeStr)
		}
		ret[date.UTC().String()] = volume
	}
	return ret, nil
}

func (service *mimoService) volumesInPast24Hours(exchanges []string) (map[string]*big.Int, error) {
	if len(exchanges) == 0 {
		return nil, nil
	}
	tempTable := "SELECT '" + exchanges[0] + "' AS `account`"
	for i := 1; i < len(exchanges); i++ {
		tempTable += " UNION SELECT '" + exchanges[i] + "'"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT e.account, SUM(t.amount) v "+
			"FROM `"+accountbalance.TransactionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"INNER JOIN ("+tempTable+") AS e "+
			"ON CONVERT(e.account USING latin1) = t.sender OR CONVERT(e.account USING latin1) = t.recipient "+
			"WHERE bi.timestamp >= ? "+
			"GROUP BY e.account",
		time.Now().UTC().Add(-24*time.Hour).Unix(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query volumes")
	}
	ret := map[string]*big.Int{}
	for rows.Next() {
		var exchange, volumeStr string
		if err := rows.Scan(&exchange, &volumeStr); err != nil {
			return nil, errors.Wrap(err, "failed to parse volume information")
		}
		volume, ok := new(big.Int).SetString(volumeStr, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse volume %s", volumeStr)
		}
		ret[exchange] = volume
	}
	return ret, nil
}

func (service *mimoService) tokens(height uint64, addrs []string) (map[string]Token, error) {
	args := make([]interface{}, len(addrs)+1)
	args[0] = height
	questionMarks := make([]string, len(addrs))
	for i, addr := range addrs {
		args[i+1] = addr
		questionMarks[i] = "?"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT `token`, `token_name`, `token_symbol` "+
			"FROM `"+mimoprotocol.ExchangeCreationTableName+"` "+
			"WHERE `block_height` < ? AND `token` in ("+strings.Join(questionMarks, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	ret := map[string]Token{}
	for rows.Next() {
		var token, name, symbol string
		if err := rows.Scan(&token, &name, &symbol); err != nil {
			return nil, err
		}
		ret[token] = Token{
			Address: token,
			Name:    name,
			Symbol:  symbol,
		}
	}
	return ret, nil
}

func (service *mimoService) balances(height uint64, addrs []string) (map[string]*big.Int, error) {
	args := make([]interface{}, len(addrs)+1)
	args[0] = height
	questionMarks := make([]string, len(addrs))
	for i, addr := range addrs {
		args[i+1] = addr
		questionMarks[i] = "?"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT b1.account, b1.balance "+
			"FROM `"+accountbalance.BalanceTableName+"` b1 INNER JOIN ("+
			"    SELECT account, MAX(block_height) max_height "+
			"    FROM `"+accountbalance.BalanceTableName+"` "+
			"    WHERE `block_height` <= ? AND account in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY account"+
			") h1 ON b1.account = h1.account and b1.block_height = h1.max_height",
		args...)
	if err != nil {
		return nil, err
	}
	ret := map[string]*big.Int{}
	for rows.Next() {
		var account string
		var balance string
		if err := rows.Scan(&account, &balance); err != nil {
			return nil, errors.Wrap(err, "failed to parse balance")
		}
		bb, ok := new(big.Int).SetString(balance, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance %s", balance)
		}
		ret[account] = bb
	}
	return ret, nil
}

func (service *mimoService) tokenBalances(height uint64, tokenAndAddrPairs []AddressPair) (map[AddressPair]*big.Int, error) {
	args := make([]interface{}, len(tokenAndAddrPairs)*2+1)
	args[0] = height
	questionMarks := make([]string, len(tokenAndAddrPairs))
	for i, pair := range tokenAndAddrPairs {
		args[2*i+1] = pair.Address1
		args[2*i+2] = pair.Address2
		questionMarks[i] = "(?,?)"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT h1.token,h1.account,b1.balance "+
			"FROM `"+xrc20.BalanceTableName+"` b1 INNER JOIN ("+
			"    SELECT token,account,MAX(block_height) max_height "+
			"    FROM `"+xrc20.BalanceTableName+"` "+
			"    WHERE `block_height` <= ? AND (`token`,`account`) in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY token,account"+
			") h1 ON b1.token = h1.token AND b1.account = h1.account AND b1.block_height = h1.max_height",
		args...)
	if err != nil {
		return nil, err
	}
	ret := map[AddressPair]*big.Int{}
	for rows.Next() {
		var token string
		var account string
		var balance string
		if err := rows.Scan(&token, &account, &balance); err != nil {
			return nil, errors.Wrap(err, "failed to parse balance")
		}
		bb, ok := new(big.Int).SetString(balance, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance %s", balance)
		}
		ret[AddressPair{token, account}] = bb
	}
	return ret, nil
}
func (service *mimoService) supplies(height uint64, tokens []string) (map[string]*big.Int, error) {
	args := make([]interface{}, len(tokens)+1)
	args[0] = height
	questionMarks := make([]string, len(tokens))
	for i, token := range tokens {
		args[i+1] = token
		questionMarks[i] = "?"
	}

	rows, err := service.store.GetDB().Query(
		"SELECT b1.token, b1.supply "+
			"FROM `"+xrc20.SupplyTableName+"` b1 "+
			"INNER JOIN ("+
			"    SELECT token, MAX(block_height) max_height "+
			"    FROM `"+xrc20.SupplyTableName+"` "+
			"    WHERE block_height <= ? AND token in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY token"+
			") h1 ON b1.token = h1.token AND b1.block_height = h1.max_height",
		args...,
	)
	if err != nil {
		return nil, err
	}
	ret := map[string]*big.Int{}
	for rows.Next() {
		var token, supply string
		if err := rows.Scan(&token, &supply); err != nil {
			return nil, errors.Wrap(err, "failed to parse supply query result")
		}
		s, ok := new(big.Int).SetString(supply, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance %s", supply)
		}
		ret[token] = s
	}
	return ret, nil
}
