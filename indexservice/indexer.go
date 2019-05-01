// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"

	s "github.com/iotexproject/iotex-api/sql"
)

type (
	// BlockByAction defines the base schema of "action to block" table
	BlockByAction struct {
		ActionHash  []byte
		ReceiptHash []byte
		BlockHash   []byte
	}
	// ActionHistory defines the schema of "action history" table
	ActionHistory struct {
		UserAddress string
		ActionHash  string
	}

	// RewardHistory defines the schema of "reward history" table
	RewardHistory struct {
		EpochNumber     string
		RewardAddress   string
		BlockReward     string
		EpochReward     string
		FoundationBonus string
	}

	// ProductivityHistory defines the schema of "productivity history" table
	ProductivityHistory struct {
		EpochNumber      string
		BlockHeight      string
		Producer         string
		ExpectedProducer string
	}

	// BlockProducersHistory defines the schema of "block producers history" table
	BlockProducersHistory struct {
		EpochNumber       string
		BlockProducerList []byte
	}
)

// Indexer handles the index build for blocks
type Indexer struct {
	store                 s.Store
	numDelegates          uint64
	numCandidateDelegates uint64
	numSubEpochs          uint64
}

// RewardInfo indicates the amount of different reward types
type RewardInfo struct {
	BlockReward     *big.Int
	EpochReward     *big.Int
	FoundationBonus *big.Int
}

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// HandleBlock is an implementation of interface BlockCreationSubscriber
func (idx *Indexer) HandleBlock(blk *block.Block) error {
	return idx.BuildIndex(blk)
}

// BuildIndex builds the index for a block
func (idx *Indexer) BuildIndex(blk *block.Block) error {
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		actionToReceipt := make(map[hash.Hash256]hash.Hash256)
		grantRewardActs := make(map[hash.Hash256]bool)
		// log action index
		for _, selp := range blk.Actions {
			callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new action for sender
			if err := idx.UpdateActionHistory(tx, callerAddr.String(), selp.Hash()); err != nil {
				return errors.Wrap(err, "failed to update action to action history table")
			}
			// put new transfer for recipient
			dst, ok := selp.Destination()
			if ok {
				if err := idx.UpdateActionHistory(tx, dst, selp.Hash()); err != nil {
					return errors.Wrap(err, "failed to update action to action history table")
				}
			}
			actionToReceipt[selp.Hash()] = hash.ZeroHash256

			if _, ok := selp.Action().(*action.GrantReward); ok {
				grantRewardActs[selp.Hash()] = true
			}

			if putPollResult, ok := selp.Action().(*action.PutPollResult); ok {
				epochNumber := idx.getEpochNum(putPollResult.Height())
				candidateList := putPollResult.Candidates()
				if len(candidateList) > int(idx.numCandidateDelegates) {
					candidateList = candidateList[:idx.numCandidateDelegates]
				}
				if err := idx.UpdateBlockProducersHistory(tx, epochNumber, candidateList); err != nil {
					return errors.Wrap(err, "failed to update epoch number to block producers history table")
				}
			}
		}

		epochNum := idx.getEpochNum(blk.Height())
		// log receipt index
		for _, receipt := range blk.Receipts {
			// map receipt to action
			if _, ok := actionToReceipt[receipt.ActionHash]; !ok {
				return errors.New("failed to find the corresponding action from receipt")
			}
			actionToReceipt[receipt.ActionHash] = receipt.Hash()

			if _, ok := grantRewardActs[receipt.ActionHash]; ok {
				// Parse receipt of grant reward
				rewardInfoMap, err := idx.getRewardInfoFromReceipt(receipt)
				if err != nil {
					return errors.Wrap(err, "failed to get reward info from receipt")
				}
				// Update reward info in DB
				if err := idx.UpdateRewardHistory(tx, epochNum, rewardInfoMap); err != nil {
					return errors.Wrap(err, "failed to update epoch number and reward address to reward history table")
				}
			}
		}
		if err := idx.UpdateBlockByAction(tx, actionToReceipt, blk.HashBlock()); err != nil {
			return errors.Wrap(err, "failed to update action index to block")
		}

		if err := idx.UpdateProductivityHistory(tx, epochNum, blk.Height(), blk.ProducerAddress()); err != nil {
			return errors.Wrapf(err, "failed to update epoch number to productivity history table")
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// UpdateBlockByAction maps action hash/receipt hash to block hash
func (idx *Indexer) UpdateBlockByAction(tx *sql.Tx, actionToReceipt map[hash.Hash256]hash.Hash256,
	blockHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (action_hash,receipt_hash,block_hash) VALUES (?, ?, ?)",
		idx.getBlockByActionTableName())
	for actionHash, receiptHash := range actionToReceipt {
		if _, err := tx.Exec(insertQuery, hex.EncodeToString(actionHash[:]), hex.EncodeToString(receiptHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateActionHistory stores action information into action history table
func (idx *Indexer) UpdateActionHistory(tx *sql.Tx, userAddr string,
	actionHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (user_address,action_hash) VALUES (?, ?)",
		idx.getActionHistoryTableName())
	if _, err := tx.Exec(insertQuery, userAddr, actionHash[:]); err != nil {
		return err
	}
	return nil
}

// UpdateRewardHistory stores reward information into reward history table
func (idx *Indexer) UpdateRewardHistory(tx *sql.Tx, epochNum uint64, rewardInfoMap map[string]*RewardInfo) error {
	for rewardAddress, rewardDelta := range rewardInfoMap {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number,reward_address,block_reward,epoch_reward,"+
			"foundation_bonus) VALUES (?, ?, ?, ?, ?)", idx.getRewardHistoryTableName())
		epochNumber := strconv.Itoa(int(epochNum))
		blockReward := rewardDelta.BlockReward.String()
		epochReward := rewardDelta.EpochReward.String()
		foundationBonus := rewardDelta.FoundationBonus.String()
		if _, err := tx.Exec(insertQuery, epochNumber, rewardAddress, blockReward, epochReward, foundationBonus); err != nil {
			return err
		}
	}
	return nil
}

// UpdateBlockProducersHistory stores block producers information into block producers history table
func (idx *Indexer) UpdateBlockProducersHistory(tx *sql.Tx, epochNum uint64, blockProducerList state.CandidateList) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number, block_producer_list) VALUES (?, ?)",
		idx.getBlockProducersHistoryTableName())
	epochNumber := strconv.Itoa(int(epochNum))
	blockProducers, err := blockProducerList.Serialize()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(insertQuery, epochNumber, blockProducers); err != nil {
		return err
	}
	return nil
}

// UpdateProductivityHistory stores block producers' productivity information into productivity history table
func (idx *Indexer) UpdateProductivityHistory(tx *sql.Tx, epochNum uint64, blockHeight uint64, blockProducer string) error {
	blockProducerList, err := idx.getBlockProducersHistory(epochNum)
	if err != nil {
		return err
	}
	blockProducerAddrs := make([]string, 0)
	for _, delegate := range blockProducerList {
		blockProducerAddrs = append(blockProducerAddrs, delegate.Address)
	}
	crypto.SortCandidates(blockProducerAddrs, epochNum, crypto.CryptoSeed)
	activeProducers := blockProducerAddrs
	if len(activeProducers) > int(idx.numDelegates) {
		activeProducers = activeProducers[:idx.numDelegates]
	}
	expectedProducer := activeProducers[int(blockHeight)%len(activeProducers)]

	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number, block_height, producer, expected_producer) "+
		"VALUES (?, ?, ?, ?)", idx.getProductivityHistoryTableName())
	epochNumber := strconv.Itoa(int(epochNum))
	height := strconv.Itoa(int(blockHeight))

	if _, err := tx.Exec(insertQuery, epochNumber, height, blockProducer, expectedProducer); err != nil {
		return err
	}
	return nil
}

// GetActionHistory returns list of action hash by user address
func (idx *Indexer) GetActionHistory(userAddr string) ([]hash.Hash256, error) {
	db := idx.store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE user_address=?",
		idx.getActionHistoryTableName())
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(userAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var actionHistory ActionHistory
	parsedRows, err := s.ParseSQLRows(rows, &actionHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	actionHashes := make([]hash.Hash256, 0, len(parsedRows))
	for _, parsedRow := range parsedRows {
		var hash hash.Hash256
		copy(hash[:], parsedRow.(*ActionHistory).ActionHash)
		actionHashes = append(actionHashes, hash)
	}
	return actionHashes, nil
}

// GetBlockByAction returns block hash by action hash
func (idx *Indexer) GetBlockByAction(actionHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE action_hash=?",
		idx.getBlockByActionTableName())
	return idx.blockByIndex(getQuery, actionHash)
}

// GetBlockByReceipt returns block hash by receipt hash
func (idx *Indexer) GetBlockByReceipt(receiptHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE receipt_hash=?",
		idx.getBlockByActionTableName())
	return idx.blockByIndex(getQuery, receiptHash)
}

// GetRewardHistory returns reward information by epoch number and reward address
func (idx *Indexer) GetRewardHistory(epochNumber uint64, rewardAddress string) (*RewardInfo, error) {
	db := idx.store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND reward_address=?",
		idx.getRewardHistoryTableName())
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	epochNumStr := strconv.Itoa(int(epochNumber))
	rows, err := stmt.Query(epochNumStr, rewardAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var rewardHistory RewardHistory
	parsedRows, err := s.ParseSQLRows(rows, &rewardHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, ErrNotExist
	}

	rewardInfo := &RewardInfo{
		BlockReward:     big.NewInt(0),
		EpochReward:     big.NewInt(0),
		FoundationBonus: big.NewInt(0),
	}
	for _, parsedRow := range parsedRows {
		rewards := parsedRow.(*RewardHistory)
		blockReward, _ := big.NewInt(0).SetString(rewards.BlockReward, 10)
		epochReward, _ := big.NewInt(0).SetString(rewards.EpochReward, 10)
		foundationBonus, _ := big.NewInt(0).SetString(rewards.FoundationBonus, 10)
		rewardInfo.BlockReward.Add(rewardInfo.BlockReward, blockReward)
		rewardInfo.EpochReward.Add(rewardInfo.EpochReward, epochReward)
		rewardInfo.FoundationBonus.Add(rewardInfo.FoundationBonus, foundationBonus)
	}
	return rewardInfo, nil
}

// GetBlockProducersHistory returns block producers information by epoch number
func (idx *Indexer) GetBlockProducersHistory(epochNumber uint64) (state.CandidateList, error) {
	return idx.getBlockProducersHistory(epochNumber)
}

// GetProductivityHistory returns productivity information by epoch number and user address
func (idx *Indexer) GetProductivityHistory(epochNumber uint64, address string) (uint64, uint64, error) {
	db := idx.store.GetDB()

	epochNumStr := strconv.Itoa(int(epochNumber))

	getProductionQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE epoch_number=? AND producer=?",
		idx.getProductivityHistoryTableName())
	stmt, err := db.Prepare(getProductionQuery)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to prepare get query")
	}

	var productions uint64
	err = stmt.QueryRow(epochNumStr, address).Scan(&productions)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to execute get query")
	}

	getExpectedProductionQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE epoch_number=? AND expected_producer=?",
		idx.getProductivityHistoryTableName())
	stmt, err = db.Prepare(getExpectedProductionQuery)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to prepare get query")
	}

	var expectedProductions uint64
	err = stmt.QueryRow(epochNumStr, address).Scan(&expectedProductions)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to execute get query")
	}

	return productions, expectedProductions, nil
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	// create block by action table
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([action_hash] BLOB(32) NOT NULL, [receipt_hash] BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)", idx.getBlockByActionTableName())); err != nil {
		return err
	}

	// create action history table
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([user_address] TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)", idx.getActionHistoryTableName())); err != nil {
		return err
	}

	// create reward history table
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [reward_address] TEXT NOT NULL, [block_reward] TEXT NOT NULL, "+
		"[epoch_reward] TEXT NOT NULL, [foundation_bonus] TEXT NOT NULL)", idx.getRewardHistoryTableName())); err != nil {
		return err
	}

	// create productivity history table
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [block_height] TEXT NOT NULL, [producer] TEXT NOT NULL, "+
		"[expected_producer] TEXT NOT NULL)",
		idx.getProductivityHistoryTableName())); err != nil {
		return err
	}

	// create block producers history table
	if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [block_producer_list] BLOB(32) NOT NULL)",
		idx.getBlockProducersHistoryTableName())); err != nil {
		return err
	}
	return nil
}

// blockByIndex returns block by index hash
func (idx *Indexer) blockByIndex(getQuery string, indexHash hash.Hash256) (hash.Hash256, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(hex.EncodeToString(indexHash[:]))
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to execute get query")
	}

	var blockByAction BlockByAction
	parsedRows, err := s.ParseSQLRows(rows, &blockByAction)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash256, ErrNotExist
	}

	var hash hash.Hash256
	copy(hash[:], parsedRows[0].(*BlockByAction).BlockHash)
	return hash, nil
}

func (idx *Indexer) getBlockProducersHistory(epochNumber uint64) (state.CandidateList, error) {
	db := idx.store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=?",
		idx.getBlockProducersHistoryTableName())
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	epochNumStr := strconv.Itoa(int(epochNumber))
	rows, err := stmt.Query(epochNumStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var blockProducersHistory BlockProducersHistory
	parsedRows, err := s.ParseSQLRows(rows, &blockProducersHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("Only one row is expected")
	}

	blockProducers := parsedRows[0].(*BlockProducersHistory)
	var blockProducerList state.CandidateList
	if err := blockProducerList.Deserialize(blockProducers.BlockProducerList); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block producer list")
	}

	return blockProducerList, nil
}

func (idx *Indexer) getBlockByActionTableName() string {
	return fmt.Sprint("block_by_action")
}

func (idx *Indexer) getActionHistoryTableName() string {
	return fmt.Sprint("action_history")
}

func (idx *Indexer) getRewardHistoryTableName() string {
	return fmt.Sprint("reward_history")
}

func (idx *Indexer) getProductivityHistoryTableName() string {
	return fmt.Sprint("productivity_history")
}

func (idx *Indexer) getBlockProducersHistoryTableName() string {
	return fmt.Sprint("block_producers_history")
}

func (idx *Indexer) getRewardInfoFromReceipt(receipt *action.Receipt) (map[string]*RewardInfo, error) {
	rewardInfoMap := make(map[string]*RewardInfo)
	for _, l := range receipt.Logs {
		rewardLog := &rewardingpb.RewardLog{}
		if err := proto.Unmarshal(l.Data, rewardLog); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal receipt data into reward log")
		}
		rewards, ok := rewardInfoMap[rewardLog.Addr]
		if !ok {
			rewardInfoMap[rewardLog.Addr] = &RewardInfo{
				BlockReward:     big.NewInt(0),
				EpochReward:     big.NewInt(0),
				FoundationBonus: big.NewInt(0),
			}
			rewards = rewardInfoMap[rewardLog.Addr]
		}
		amount, ok := big.NewInt(0).SetString(rewardLog.Amount, 10)
		if !ok {
			log.L().Fatal("Failed to convert reward amount from string to big int")
			return nil, errors.New("failed to convert reward amount from string to big int")
		}
		switch rewardLog.Type {
		case rewardingpb.RewardLog_BLOCK_REWARD:
			rewards.BlockReward = amount
		case rewardingpb.RewardLog_EPOCH_REWARD:
			rewards.EpochReward = amount
		case rewardingpb.RewardLog_FOUNDATION_BONUS:
			rewards.FoundationBonus = amount
		default:
			log.L().Fatal("Unknown type of reward")
		}
	}
	return rewardInfoMap, nil
}

func (idx *Indexer) getEpochNum(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	return (height-1)/idx.numDelegates/idx.numSubEpochs + 1
}
