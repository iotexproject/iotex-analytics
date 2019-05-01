// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"database/sql"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/require"

	s "github.com/iotexproject/iotex-api/sql"
)

func testSQLite3StorePutGet(store s.Store, t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	err := store.Start(ctx)
	require.Nil(err)
	defer func() {
		err = store.Stop(ctx)
		require.Nil(err)
	}()

	addr1 := testaddress.Addrinfo["alfa"].String()
	pubKey1 := testaddress.Keyinfo["alfa"].PubKey
	addr2 := testaddress.Addrinfo["bravo"].String()
	pubKey2 := testaddress.Keyinfo["bravo"].PubKey
	rewardAddr1 := testaddress.Addrinfo["charlie"].String()
	rewardAddr2 := testaddress.Addrinfo["delta"].String()
	rewardAddr3 := testaddress.Addrinfo["echo"].String()

	idx := Indexer{
		store:                 store,
		numDelegates:          24,
		numCandidateDelegates: 36,
		numSubEpochs:          15,
	}
	err = idx.CreateTablesIfNotExist()
	require.Nil(err)

	initialBPList := make(state.CandidateList, 0)
	initialBPList = append(initialBPList, &state.Candidate{
		Address:       addr1,
		Votes:         big.NewInt(10),
		RewardAddress: rewardAddr1,
	})
	// TODO: Update Block Producers information right after creating tables
	require.NoError(idx.store.Transact(func(tx *sql.Tx) error {
		return idx.UpdateBlockProducersHistory(tx, uint64(1), initialBPList)
	}))

	blockProducers, err := idx.GetBlockProducersHistory(uint64(1))
	require.NoError(err)
	require.Equal(len(initialBPList), len(blockProducers))

	blk1 := block.Block{}

	err = blk1.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    180,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: pubKey1.Bytes(),
		},
		Body: &iotextypes.BlockBody{
			Actions: []*iotextypes.Action{
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{Recipient: addr2},
						},
						Version: version.ProtocolVersion,
						Nonce:   101,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Vote{
							Vote: &iotextypes.Vote{VoteeAddress: addr2},
						},
						Version: version.ProtocolVersion,
						Nonce:   102,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Execution{
							Execution: &iotextypes.Execution{Contract: addr2},
						},
						Version: version.ProtocolVersion,
						Nonce:   103,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_PutPollResult{
							PutPollResult: &iotextypes.PutPollResult{
								Height: 361,
								Candidates: &iotextypes.CandidateList{
									Candidates: []*iotextypes.Candidate{
										{
											Address: addr1,
											Votes:   big.NewInt(100).Bytes(),
											PubKey:  pubKey1.Bytes(),
										},
										{
											Address: addr2,
											Votes:   big.NewInt(50).Bytes(),
											PubKey:  pubKey2.Bytes(),
										},
									},
								},
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   104,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_GrantReward{
							GrantReward: &iotextypes.GrantReward{
								Height: 180,
								Type:   iotextypes.RewardType_BlockReward,
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   105,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_GrantReward{
							GrantReward: &iotextypes.GrantReward{
								Height: 180,
								Type:   iotextypes.RewardType_EpochReward,
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   106,
					},
					SenderPubKey: pubKey1.Bytes(),
				},
			},
		},
	})
	require.NoError(err)
	receipts := []*action.Receipt{
		{
			ActionHash:      blk1.Actions[0].Hash(),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
			Logs:            []*action.Log{},
		},
		{
			ActionHash:      blk1.Actions[1].Hash(),
			Status:          2,
			GasConsumed:     2,
			ContractAddress: "2",
			Logs:            []*action.Log{},
		},
		{
			ActionHash:      blk1.Actions[2].Hash(),
			Status:          3,
			GasConsumed:     3,
			ContractAddress: "3",
			Logs:            []*action.Log{},
		},
		{
			ActionHash:  blk1.Actions[3].Hash(),
			Status:      4,
			GasConsumed: 4,
			Logs:        []*action.Log{},
		},
	}
	receipts = append(receipts, &action.Receipt{
		ActionHash:      blk1.Actions[4].Hash(),
		Status:          5,
		GasConsumed:     5,
		ContractAddress: "5",
		Logs: []*action.Log{
			createRewardLog(uint64(1), blk1.Actions[4].Hash(), rewardingpb.RewardLog_BLOCK_REWARD, rewardAddr1, "16"),
		},
	})
	receipts = append(receipts, &action.Receipt{
		ActionHash:      blk1.Actions[5].Hash(),
		Status:          6,
		GasConsumed:     6,
		ContractAddress: "6",
		Logs: []*action.Log{
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr1, "10"),
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr2, "20"),
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr3, "30"),
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, rewardAddr1, "100"),
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, rewardAddr2, "100"),
			createRewardLog(uint64(180), blk1.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, rewardAddr3, "100"),
		},
	})
	blk1.Receipts = make([]*action.Receipt, 0)
	/*for _, receipt := range receipts {
		blk.Receipts = append(blk.Receipts, receipt)
	}*/
	blk1.Receipts = append(blk1.Receipts, receipts...)

	err = idx.BuildIndex(&blk1)
	require.Nil(err)

	// get receipt
	blkHash, err := idx.GetBlockByReceipt(receipts[0].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk1.HashBlock())

	blkHash, err = idx.GetBlockByReceipt(receipts[1].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk1.HashBlock())

	// get action
	actionHashes, err := idx.GetActionHistory(addr1)
	require.Nil(err)
	require.Equal(6, len(actionHashes))
	action := blk1.Actions[0].Hash()
	require.Equal(action, actionHashes[0])

	// action map to block
	blkHash4, err := idx.GetBlockByAction(blk1.Actions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash4, blk1.HashBlock())

	blockProducers, err = idx.GetBlockProducersHistory(uint64(2))
	require.NoError(err)
	require.Equal(2, len(blockProducers))

	production, expectedProduction, err := idx.GetProductivityHistory(uint64(1), addr1)
	require.NoError(err)
	require.Equal(uint64(1), production)
	require.Equal(uint64(1), expectedProduction)

	rewardInfo, err := idx.GetRewardHistory(idx.getEpochNum(blk1.Height()), rewardAddr1)
	require.NoError(err)
	require.Equal("16", rewardInfo.BlockReward.String())
	require.Equal("10", rewardInfo.EpochReward.String())
	require.Equal("100", rewardInfo.FoundationBonus.String())

	blk2 := block.Block{}

	err = blk2.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    361,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: pubKey1.Bytes(),
		},
		Body: &iotextypes.BlockBody{},
	})
	require.NoError(err)

	err = idx.BuildIndex(&blk2)
	require.NoError(err)

	production1, expectedProduction1, err := idx.GetProductivityHistory(uint64(2), addr1)
	require.NoError(err)
	require.Equal(uint64(1), production1)
	require.Equal(uint64(0), expectedProduction1)

	production2, expectedProduction2, err := idx.GetProductivityHistory(uint64(2), addr2)
	require.NoError(err)
	require.Equal(uint64(0), production2)
	require.Equal(uint64(1), expectedProduction2)
}

func TestIndexServiceOnSqlite3(t *testing.T) {
	path := "explorer.db"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.SQLITE3.SQLite3File = testPath
	t.Run("Indexer", func(t *testing.T) {
		testSQLite3StorePutGet(s.NewSQLite3(cfg.SQLITE3), t)
	})
}

func TestIndexServiceOnAwsRDS(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	t.Run("Indexer", func(t *testing.T) {
		testSQLite3StorePutGet(s.NewAwsRDS(config.Default.DB.RDS), t)
	})
}

func createRewardLog(
	blkHeight uint64,
	actionHash hash.Hash256,
	rewardType rewardingpb.RewardLog_RewardType,
	rewardAddr string,
	amount string,
) *action.Log {
	h := hash.Hash160b([]byte("rewarding"))
	addr, _ := address.FromBytes(h[:])
	log := &action.Log{
		Address:     addr.String(),
		Topics:      nil,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}

	rewardData := rewardingpb.RewardLog{
		Type:   rewardType,
		Addr:   rewardAddr,
		Amount: amount,
	}

	data, _ := proto.Marshal(&rewardData)
	log.Data = data
	return log
}
