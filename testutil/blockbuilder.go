// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	// Addr1 is a testing address
	Addr1 = identityset.Address(0).String()
	// PubKey1 is a testing public key
	PubKey1 = identityset.PrivateKey(0).PublicKey()
	// Addr2 is a testing address
	Addr2 = identityset.Address(1).String()
	// PubKey2 is testing public key
	PubKey2 = identityset.PrivateKey(1).PublicKey()
	// RewardAddr1 is a testing reward address
	RewardAddr1 = identityset.Address(2).String()
	// RewardAddr2 is a testing reward address
	RewardAddr2 = identityset.Address(3).String()
	// RewardAddr3 is a testing reward address
	RewardAddr3 = identityset.Address(4).String()
)

// BuildCompleteBlock builds a complete block
func BuildCompleteBlock(height uint64, nextEpochHeight uint64) (*block.Block, error) {
	blk := block.Block{}

	if err := blk.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    height,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: PubKey1.Bytes(),
		},
		Body: &iotextypes.BlockBody{
			Actions: []*iotextypes.Action{
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{Recipient: Addr1, Amount: "1"},
						},
						Version: version.ProtocolVersion,
						Nonce:   101,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{Recipient: Addr2, Amount: "2"},
						},
						Version: version.ProtocolVersion,
						Nonce:   102,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Execution{
							Execution: &iotextypes.Execution{Contract: Addr2},
						},
						Version: version.ProtocolVersion,
						Nonce:   103,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_PutPollResult{
							PutPollResult: &iotextypes.PutPollResult{
								Height: nextEpochHeight,
								Candidates: &iotextypes.CandidateList{
									Candidates: []*iotextypes.Candidate{
										{
											Address: Addr1,
											Votes:   big.NewInt(100).Bytes(),
											PubKey:  PubKey1.Bytes(),
										},
										{
											Address: Addr2,
											Votes:   big.NewInt(50).Bytes(),
											PubKey:  PubKey2.Bytes(),
										},
									},
								},
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   104,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_GrantReward{
							GrantReward: &iotextypes.GrantReward{
								Height: height,
								Type:   iotextypes.RewardType_BlockReward,
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   105,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_GrantReward{
							GrantReward: &iotextypes.GrantReward{
								Height: height,
								Type:   iotextypes.RewardType_EpochReward,
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   106,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Execution{
							Execution: &iotextypes.Execution{},
						},
						Version: version.ProtocolVersion,
						Nonce:   107,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Execution{
							Execution: &iotextypes.Execution{},
						},
						Version: version.ProtocolVersion,
						Nonce:   108,
					},
					SenderPubKey: PubKey1.Bytes(),
				},
			},
		},
	}); err != nil {
		return nil, err
	}

	receipts := []*action.Receipt{
		{
			ActionHash:      blk.Actions[0].Hash(),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
		},
		{
			ActionHash:      blk.Actions[1].Hash(),
			Status:          1,
			GasConsumed:     2,
			ContractAddress: "2",
		},
		{
			ActionHash:      blk.Actions[2].Hash(),
			Status:          3,
			GasConsumed:     3,
			ContractAddress: "3",
		},
		{
			ActionHash:  blk.Actions[3].Hash(),
			Status:      4,
			GasConsumed: 4,
		},
	}
	testReceipt := &action.Receipt{
		ActionHash:      blk.Actions[4].Hash(),
		Status:          5,
		GasConsumed:     5,
		ContractAddress: "5"}
	testReceipt.AddLogs(
		createRewardLog(uint64(1), blk.Actions[4].Hash(), rewardingpb.RewardLog_BLOCK_REWARD, RewardAddr1, "16"))
	receipts = append(receipts, testReceipt)
	testReceipt2 := &action.Receipt{
		ActionHash:      blk.Actions[5].Hash(),
		Status:          6,
		GasConsumed:     6,
		ContractAddress: "6",
	}
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, RewardAddr1, "10"))
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, RewardAddr2, "20"))
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_EPOCH_REWARD, RewardAddr3, "30"))
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, RewardAddr1, "100"))
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, RewardAddr2, "100"))
	testReceipt2.AddLogs(createRewardLog(height, blk.Actions[5].Hash(), rewardingpb.RewardLog_FOUNDATION_BONUS, RewardAddr3, "100"))
	receipts = append(receipts, testReceipt2)
	// add for xrc20
	transferHash, _ := hex.DecodeString("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	data, _ := hex.DecodeString("0000000000000000000000006356908ace09268130dee2b7de643314bbeb3683000000000000000000000000da7e12ef57c236a06117c5e0d04a228e7181cf360000000000000000000000000000000000000000000000000de0b6b3a7640000")
	testReceipt3 := &action.Receipt{
		ActionHash:      blk.Actions[6].Hash(),
		Status:          7,
		GasConsumed:     7,
		ContractAddress: "7",
	}
	testReceipt3.AddLogs(&action.Log{
		Address:     "xxxxx",
		Topics:      []hash.Hash256{hash.BytesToHash256(transferHash)},
		Data:        data,
		BlockHeight: 100000,
		ActionHash:  blk.Actions[6].Hash(),
		Index:       888,
	})
	receipts = append(receipts, testReceipt3)

	// add for xrc721
	transferHash, _ = hex.DecodeString("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ff003f0d751d3a71172f723fbbc4d262dd47adf00000000000000000000000000000000000000000000000000000000000000006")
	testReceipt4 := &action.Receipt{
		ActionHash:      blk.Actions[7].Hash(),
		Status:          8,
		GasConsumed:     8,
		ContractAddress: "888",
	}
	testReceipt4.AddLogs(&action.Log{
		Address: "io1xpvzahnl4h46f9ea6u03ec2hkusrzu020th8xx",
		Topics: []hash.Hash256{
			hash.BytesToHash256(transferHash[:32]),
			hash.BytesToHash256(transferHash[32:64]),
			hash.BytesToHash256(transferHash[64:96]),
			hash.BytesToHash256(transferHash[96:128]),
		},
		BlockHeight: 100001,
		ActionHash:  blk.Actions[7].Hash(),
		Index:       666,
	})
	receipts = append(receipts, testReceipt4)

	blk.Receipts = make([]*action.Receipt, 0)
	/*for _, receipt := range receipts {
		blk.Receipts = append(blk.Receipts, receipt)
	}*/
	blk.Receipts = append(blk.Receipts, receipts...)

	return &blk, nil
}

// BuildEmptyBlock builds an empty block
func BuildEmptyBlock(height uint64) (*block.Block, error) {
	blk := block.Block{}

	if err := blk.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    height,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: PubKey1.Bytes(),
		},
		Body: &iotextypes.BlockBody{},
	}); err != nil {
		return nil, err
	}
	return &blk, nil
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
