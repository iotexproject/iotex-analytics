// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexprotocol

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// PollProtocolID is ID of poll protocol
	PollProtocolID      = "poll"
	protocolID          = "staking"
	readBucketsLimit    = 30000
	readCandidatesLimit = 20000
)

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
	// ErrUnimplemented indicates a method is not implemented yet
	ErrUnimplemented = errors.New("method is unimplemented")
)

// Genesis defines the genesis configurations that should be recorded by the corresponding protocol before
// indexing the first block
type (
	Genesis struct {
		Account `yaml:"account"`
	}
	// Account contains the configs for account protocol
	Account struct {
		// InitBalanceMap is the address and initial balance mapping before the first block.
		InitBalanceMap map[string]string `yaml:"initBalances"`
	}
	//Poll contains the configs for voting protocol
	Poll struct {
		SkipManifiedCandidate bool   `yaml:"skipManifiedCandidate"`
		VoteThreshold         string `yaml:"voteThreshold"`
		ScoreThreshold        string `yaml:"scoreThreshold"`
		SelfStakingThreshold  string `yaml:"selfStakingThreshold"`
	}
	// GravityChain contains the configs for gravity chain
	GravityChain struct {
		GravityChainStartHeight     uint64   `yaml:"gravityChainStartHeight"`
		GravityChainAPIs            []string `yaml:"gravityChainAPIs"`
		RegisterContractAddress     string   `yaml:"registerContractAddress"`
		RewardPercentageStartHeight uint64   `yaml:"rewardPercentageStartHeight"`
	}
	// Rewarding contains the configs for rewarding
	Rewarding struct {
		NumDelegatesForEpochReward      uint64   `yaml:"numDelegatesForEpochReward"`
		NumDelegatesForFoundationBonus  uint64   `yaml:"numDelegatesForFoundationBonus"`
		FoundationBonusLastEpoch        uint64   `yaml:"foundationBonusLastEpoch"`
		ProductivityThreshold           uint64   `yaml:"productivityThreshold"`
		ExemptCandidatesFromEpochReward []string `yaml:"exemptCandidatesFromEpochReward"`
	}
	// HermesConfig defines hermes addr
	HermesConfig struct {
		HermesContractAddress    string `yaml:"hermesContractAddress"`
		MultiSendContractAddress string `yaml:"multiSendContractAddress"`
	}
	// VoteWeightCalConsts is for staking v2
	VoteWeightCalConsts struct {
		DurationLg float64 `yaml:"durationLg"`
		AutoStake  float64 `yaml:"autoStake"`
		SelfStake  float64 `yaml:"selfStake"`
	}
)

// Protocol defines the protocol interfaces for block indexer
type Protocol interface {
	BlockHandler
	CreateTables(context.Context) error
	Initialize(context.Context, *sql.Tx, *Genesis) error
}

// BlockHandler is the interface of handling block
type BlockHandler interface {
	HandleBlock(context.Context, *sql.Tx, *block.Block) error
}

func GetBucketsAllV2(chainClient iotexapi.APIServiceClient, height uint64) (voteBucketListAll *iotextypes.VoteBucketList, err error) {
	voteBucketListAll = &iotextypes.VoteBucketList{}
	for i := uint32(0); ; i++ {
		offset := i * readBucketsLimit
		size := uint32(readBucketsLimit)
		voteBucketList, err := GetBucketsV2(chainClient, offset, size, height)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get bucket")
		}
		voteBucketListAll.Buckets = append(voteBucketListAll.Buckets, voteBucketList.Buckets...)
		if len(voteBucketList.Buckets) < readBucketsLimit {
			break
		}
	}
	return
}

func GetBucketsV2(chainClient iotexapi.APIServiceClient, offset, limit uint32, height uint64) (voteBucketList *iotextypes.VoteBucketList, err error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(protocolID),
		MethodName: methodName,
		Arguments:  [][]byte{arg, []byte(strconv.FormatUint(height, 10))},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// TODO rm this when commit pr
			fmt.Println("ReadStakingDataMethod_BUCKETS not found")
		}
		return
	}
	voteBucketList = &iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(readStateRes.GetData(), voteBucketList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal VoteBucketList")
	}
	return
}

func GetCandidatesAllV2(chainClient iotexapi.APIServiceClient, height uint64) (candidateListAll *iotextypes.CandidateListV2, err error) {
	candidateListAll = &iotextypes.CandidateListV2{}
	for i := uint32(0); ; i++ {
		offset := i * readCandidatesLimit
		size := uint32(readCandidatesLimit)
		candidateList, err := GetCandidatesV2(chainClient, offset, size, height)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get candidates")
		}
		candidateListAll.Candidates = append(candidateListAll.Candidates, candidateList.Candidates...)
		if len(candidateList.Candidates) < readCandidatesLimit {
			break
		}
	}
	return
}

func GetCandidatesV2(chainClient iotexapi.APIServiceClient, offset, limit uint32, height uint64) (candidateList *iotextypes.CandidateListV2, err error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(protocolID),
		MethodName: methodName,
		Arguments:  [][]byte{arg, []byte(strconv.FormatUint(height, 10))},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// TODO rm this when commit pr
			fmt.Println("ReadStakingDataMethod_CANDIDATES not found")
		}
		return
	}
	candidateList = &iotextypes.CandidateListV2{}
	if err := proto.Unmarshal(readStateRes.GetData(), candidateList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal VoteBucketList")
	}
	return
}
