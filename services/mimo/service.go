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

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blockinfo"
	mimoprotocol "github.com/iotexproject/iotex-analytics/indexprotocol/mimo"
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
			// WithTransactionLogs: true,
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
			if err := service.buildIndex(ctx, &indexprotocol.BlockData{
				Block: blk,
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

func (service *mimoService) numOfPairs() (int, error) {
	var n int
	if err := service.store.GetDB().QueryRow("SELECT count(exchange) FROM `" + mimoprotocol.ExchangeCreationTableName + "`").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (service *mimoService) exchange(height uint64, exchange string) (AddressPair, error) {
	pair := AddressPair{
		Address1: exchange,
	}
	var token sql.NullString
	if err := service.store.GetDB().QueryRow(
		"SELECT `token` FROM `"+mimoprotocol.ExchangeCreationTableName+"` WHERE `block_height` <= ? AND `exchange` = ?",
		height,
		exchange,
	).Scan(&token); err != nil {
		return pair, err
	}
	if !token.Valid {
		return pair, errors.Errorf("failed to query exchange %s", exchange)
	}
	pair.Address2 = token.String
	return pair, nil
}

func (service *mimoService) exchanges(height uint64, offset uint32, count uint8) ([]AddressPair, error) {
	rows, err := service.store.GetDB().Query(
		"SELECT `exchange`, `token` FROM `"+mimoprotocol.ExchangeCreationTableName+"` WHERE `block_height` <= ? ORDER BY `block_height` LIMIT ?,?",
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

func (service *mimoService) numOfTransactions(duration time.Duration) (int, error) {
	var count int
	err := service.store.GetDB().QueryRow(
		"SELECT COUNT(DISTINCT(t.action_hash)) "+
			"FROM `"+mimoprotocol.ExchangeActionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"WHERE bi.timestamp >= ? ",
		time.Now().UTC().Add(-duration).String(),
	).Scan(&count)

	return count, err
}

func (service *mimoService) volumeOfAll(duration time.Duration) (*big.Int, error) {
	var s sql.NullString
	if err := service.store.GetDB().QueryRow(
		"SELECT SUM(t.iotx_amount) v "+
			"FROM `"+mimoprotocol.ExchangeActionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"WHERE bi.timestamp >= ? AND t.type in ("+mimoprotocol.JoinTopicsWithQuotes(mimoprotocol.CoinPurchase, mimoprotocol.TokenPurchase)+")",
		time.Now().UTC().Add(-duration).String(),
	).Scan(&s); err != nil {
		return nil, errors.Wrap(err, "failed to query volumes")
	}
	if !s.Valid {
		return big.NewInt(0), nil
	}
	volume, ok := new(big.Int).SetString(s.String, 10)
	if !ok {
		return nil, errors.Errorf("failed to parse volume %+v", s)
	}
	return volume, nil
}

func (service *mimoService) volumesInPastNDays(exchanges []string, days uint8) ([]time.Time, []*big.Int, error) {
	if days == 0 {
		days = 1
	}
	args := make([]interface{}, len(exchanges)+1)
	args[0] = time.Now().UTC().Add(-time.Duration(days-1) * 24 * time.Hour).Truncate(24 * time.Hour).String()
	questionMarks := make([]string, len(exchanges))
	for i, exchange := range exchanges {
		questionMarks[i] = "?"
		args[i+1] = exchange
	}
	exchangeCondition := ""
	if len(exchanges) > 0 {
		exchangeCondition = " AND t.exchange in (" + strings.Join(questionMarks, ",") + ")"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT DATE(bi.timestamp) d, SUM(t.iotx_amount) v "+
			"FROM `"+mimoprotocol.ExchangeActionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"WHERE bi.timestamp >= ?"+exchangeCondition+" AND t.type in ("+mimoprotocol.JoinTopicsWithQuotes(mimoprotocol.AddLiquidity, mimoprotocol.RemoveLiquidity)+") "+
			"GROUP BY d "+
			"ORDER BY d",
		args...,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query volumes")
	}
	dates := []time.Time{}
	volumes := []*big.Int{}
	for rows.Next() {
		var date time.Time
		var volumeStr string
		if err := rows.Scan(&date, &volumeStr); err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse volume information")
		}
		volume, ok := new(big.Int).SetString(volumeStr, 10)
		if !ok {
			return nil, nil, errors.Errorf("failed to parse volume %s", volumeStr)
		}
		dates = append(dates, date)
		volumes = append(volumes, volume)
	}
	return dates, volumes, nil
}

const layoutISO = "2006-01-02"

func (service *mimoService) liquiditiesInPastNDays(exchanges []string, days uint8) ([]time.Time, []*big.Int, error) {
	if days == 0 {
		days = 1
	}
	now := time.Now().UTC()
	tempTable := "SELECT '" + now.Format(layoutISO) + "' AS `date`"
	for i := uint8(1); i < days; i++ {
		tempTable += " UNION SELECT '" + now.Add(-time.Duration(i)*24*time.Hour).Format(layoutISO) + "'"
	}
	args := make([]interface{}, len(exchanges))
	questionMarks := make([]string, len(exchanges))
	for i, exchange := range exchanges {
		questionMarks[i] = "?"
		args[i] = exchange
	}
	exchangeCondition := ""
	if len(exchanges) > 0 {
		exchangeCondition = "WHERE ab.exchange in (" + strings.Join(questionMarks, ",") + ")"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT h1.date, SUM(b1.balance) * 2 "+
			"FROM `"+mimoprotocol.CoinBalanceTableName+"` b1 INNER JOIN ("+
			"    SELECT ab.exchange, t.date, MAX(bi.block_height) max_height "+
			"    FROM `"+mimoprotocol.CoinBalanceTableName+"` ab "+
			"    INNER JOIN `"+blockinfo.TableName+"` bi "+
			"    ON bi.block_height = ab.block_height "+
			"    INNER JOIN ("+tempTable+") AS t "+
			"    ON Date(bi.timestamp) <= t.date "+exchangeCondition+
			"    GROUP BY ab.exchange, t.date "+
			") h1 ON b1.exchange = h1.exchange AND b1.block_height = h1.max_height "+
			"GROUP BY h1.date "+
			"ORDER BY h1.date",
		args...,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to query balances")
	}
	dates := []time.Time{}
	liquidities := []*big.Int{}
	for rows.Next() {
		var date, liquidity string
		if err := rows.Scan(&date, &liquidity); err != nil {
			return nil, nil, errors.Wrap(err, "failed to scan result")
		}
		d, err := time.ParseInLocation(layoutISO, date, time.UTC)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse date %s", d)
		}
		l, ok := new(big.Int).SetString(liquidity, 10)
		if !ok {
			return nil, nil, errors.Errorf("failed to parse liquidity %s", liquidity)
		}
		dates = append(dates, d)
		liquidities = append(liquidities, l)
	}
	return dates, liquidities, nil
}

func (service *mimoService) volumes(exchanges []string, duration time.Duration) (map[string]*big.Int, error) {
	if len(exchanges) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(exchanges)+1)
	args[0] = time.Now().UTC().Add(-duration).String()
	questionMarks := make([]string, len(exchanges))
	for i, exchange := range exchanges {
		args[i+1] = exchange
		questionMarks[i] = "?"
	}

	rows, err := service.store.GetDB().Query(
		"SELECT t.exchange, SUM(t.iotx_amount) v "+
			"FROM `"+mimoprotocol.ExchangeActionTableName+"` t "+
			"INNER JOIN `"+blockinfo.TableName+"` bi "+
			"ON bi.block_height = t.block_height "+
			"WHERE bi.timestamp >= ? AND t.exchange in ("+strings.Join(questionMarks, ",")+") AND t.type in ("+mimoprotocol.JoinTopicsWithQuotes(mimoprotocol.CoinPurchase, mimoprotocol.TokenPurchase)+") "+
			"GROUP BY t.exchange",
		args...,
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
		"SELECT `token`,`token_name`,`token_symbol`,`token_decimals` "+
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
		var decimals int
		if err := rows.Scan(&token, &name, &symbol, &decimals); err != nil {
			return nil, err
		}
		ret[token] = Token{
			Address:  token,
			Decimals: decimals,
			Name:     name,
			Symbol:   symbol,
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
		"SELECT b1.exchange, b1.balance "+
			"FROM `"+mimoprotocol.CoinBalanceTableName+"` b1 INNER JOIN ("+
			"    SELECT exchange, MAX(block_height) max_height "+
			"    FROM `"+mimoprotocol.CoinBalanceTableName+"` "+
			"    WHERE `block_height` <= ? AND exchange in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY exchange"+
			") h1 ON b1.exchange = h1.exchange and b1.block_height = h1.max_height",
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

func (service *mimoService) tokenBalances(height uint64, exchanges []string) (map[string]*big.Int, error) {
	args := make([]interface{}, len(exchanges)+1)
	args[0] = height
	questionMarks := make([]string, len(exchanges))
	for i, exchange := range exchanges {
		args[i+1] = exchange
		questionMarks[i] = "?"
	}
	rows, err := service.store.GetDB().Query(
		"SELECT h1.exchange,b1.balance "+
			"FROM `"+mimoprotocol.TokenBalanceTableName+"` b1 INNER JOIN ("+
			"    SELECT `exchange`,MAX(`block_height`) max_height "+
			"    FROM `"+mimoprotocol.TokenBalanceTableName+"` "+
			"    WHERE `block_height` <= ? AND `exchange` in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY `exchange`"+
			") h1 ON b1.exchange = h1.exchange AND b1.block_height = h1.max_height",
		args...)
	if err != nil {
		return nil, err
	}
	ret := map[string]*big.Int{}
	for rows.Next() {
		var exchange string
		var balance string
		if err := rows.Scan(&exchange, &balance); err != nil {
			return nil, errors.Wrap(err, "failed to parse balance")
		}
		bb, ok := new(big.Int).SetString(balance, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance %s", balance)
		}
		ret[exchange] = bb
	}
	return ret, nil
}

func (service *mimoService) supplies(height uint64, exchanges []string) (map[string]*big.Int, error) {
	args := make([]interface{}, len(exchanges)+1)
	args[0] = height
	questionMarks := make([]string, len(exchanges))
	for i, exchange := range exchanges {
		args[i+1] = exchange
		questionMarks[i] = "?"
	}

	rows, err := service.store.GetDB().Query(
		"SELECT b1.exchange, b1.supply "+
			"FROM `"+mimoprotocol.SupplyTableName+"` b1 "+
			"INNER JOIN ("+
			"    SELECT `exchange`, MAX(`block_height`) max_height "+
			"    FROM `"+mimoprotocol.SupplyTableName+"` "+
			"    WHERE `block_height` <= ? AND `exchange` in ("+strings.Join(questionMarks, ",")+")"+
			"    GROUP BY `exchange`"+
			") h1 ON b1.exchange = h1.exchange AND b1.block_height = h1.max_height",
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

func (service *mimoService) actions(exchanges []string, topics []mimoprotocol.EventTopic, skip, first int) ([]*Action, error) {
	if first > 512 {
		return nil, errors.New("page size cannot be more than 512")
	}
	questionMarks := make([]string, len(exchanges))
	args := make([]interface{}, len(exchanges)+2)
	for i, exchange := range exchanges {
		questionMarks[i] = "?"
		args[i] = exchange
	}
	args[len(exchanges)] = skip
	args[len(exchanges)+1] = first
	exchangeCondition := ""
	if len(exchanges) > 0 {
		exchangeCondition = "AND a.exchange in (" + strings.Join(questionMarks, ",") + ") "
	}
	rows, err := service.store.GetDB().Query(
		"SELECT a.action_hash,a.type,a.exchange,a.provider,a.iotx_amount,a.token_amount,b.timestamp "+
			"FROM `"+mimoprotocol.ExchangeActionTableName+"` a "+
			"LEFT JOIN `"+blockinfo.TableName+"` b "+
			"ON a.block_height = b.block_height "+
			"WHERE a.type in ("+mimoprotocol.JoinTopicsWithQuotes(topics...)+") "+exchangeCondition+
			"ORDER BY a.block_height DESC "+
			"LIMIT ?,?",
		args...,
	)
	if err != nil {
		return nil, err
	}
	actions := []*Action{}
	for rows.Next() {
		var actionHash, actionType, exchangeAddr, actor, iotxAmount, tokenAmount string
		var date time.Time
		if err := rows.Scan(&actionHash, &actionType, &exchangeAddr, &actor, &iotxAmount, &tokenAmount, &date); err != nil {
			return nil, errors.Wrap(err, "failed to parse supply query result")
		}
		actions = append(actions, &Action{
			Type:        convertActionType(actionType),
			Exchange:    exchangeAddr,
			Account:     actor,
			IotxAmount:  iotxAmount,
			TokenAmount: tokenAmount,
			Time:        date.String(),
		})
	}
	return actions, nil
}

func convertActionType(at string) ActionType {
	switch mimoprotocol.EventTopic(at) {
	case mimoprotocol.AddLiquidity:
		return ActionTypeAdd
	case mimoprotocol.RemoveLiquidity:
		return ActionTypeRemove
	case mimoprotocol.CoinPurchase:
		fallthrough
	case mimoprotocol.TokenPurchase:
		return ActionTypeSwap
	default:
		log.L().Panic("failed to convert action type", zap.String("action type", at))
	}
	return ActionTypeAll
}
