package rewards

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-api/protocol"
	s "github.com/iotexproject/iotex-api/sql"
)

// ProtocolID is the ID of protocol
const ProtocolID = "rewards"

type (
	// RewardHistory defines the schema of "reward history" table
	RewardHistory struct {
		EpochNumber     string
		RewardAddress   string
		BlockReward     string
		EpochReward     string
		FoundationBonus string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store        s.Store
	NumDelegates uint64
	NumSubEpochs uint64
}

// RewardInfo indicates the amount of different reward types
type RewardInfo struct {
	BlockReward     *big.Int
	EpochReward     *big.Int
	FoundationBonus *big.Int
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{Store: store, NumDelegates: numDelegates, NumSubEpochs: numSubEpochs}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create reward history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [reward_address] TEXT NOT NULL, [block_reward] TEXT NOT NULL, "+
		"[epoch_reward] TEXT NOT NULL, [foundation_bonus] TEXT NOT NULL)", p.getRewardHistoryTableName())); err != nil {
		return err
	}
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	grantRewardActs := make(map[hash.Hash256]bool)
	// log action index
	for _, selp := range blk.Actions {
		if _, ok := selp.Action().(*action.GrantReward); ok {
			grantRewardActs[selp.Hash()] = true
		}
	}
	epochNum := protocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, blk.Height())
	// log receipt index
	for _, receipt := range blk.Receipts {
		if _, ok := grantRewardActs[receipt.ActionHash]; ok {
			// Parse receipt of grant reward
			rewardInfoMap, err := p.getRewardInfoFromReceipt(receipt)
			if err != nil {
				return errors.Wrap(err, "failed to get reward info from receipt")
			}
			// Update reward info in DB
			if err := p.updateRewardHistory(tx, epochNum, rewardInfoMap); err != nil {
				return errors.Wrap(err, "failed to update epoch number and reward address to reward history table")
			}
		}
	}
	return nil
}

// ReadTable reads indices in the table
func (p *Protocol) ReadTable(context.Context, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

// updateRewardHistory stores reward information into reward history table
func (p *Protocol) updateRewardHistory(tx *sql.Tx, epochNum uint64, rewardInfoMap map[string]*RewardInfo) error {
	for rewardAddress, rewardDelta := range rewardInfoMap {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number,reward_address,block_reward,epoch_reward,"+
			"foundation_bonus) VALUES (?, ?, ?, ?, ?)", p.getRewardHistoryTableName())
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

// GetRewardHistory returns reward information by epoch number and reward address
func (p *Protocol) GetRewardHistory(epochNumber uint64, rewardAddress string) (*RewardInfo, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND reward_address=?",
		p.getRewardHistoryTableName())
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
		return nil, protocol.ErrNotExist
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

func (p *Protocol) getRewardInfoFromReceipt(receipt *action.Receipt) (map[string]*RewardInfo, error) {
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

func (p *Protocol) getRewardHistoryTableName() string {
	return fmt.Sprint("reward_history")
}
