// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexprotocol

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

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
		ProductivityThreshold           uint64   `yaml:"productivityThreshold"`
		ExemptCandidatesFromEpochReward []string `yaml:"exemptCandidatesFromEpochReward"`
	}
	// HermesConfig defines hermes addr
	HermesConfig struct {
		HermesContractAddress        string   `yaml:"hermesContractAddress"`
		MultiSendContractAddressList []string `yaml:"multiSendContractAddressList"`
	}
	// VoteWeightCalConsts is for staking
	VoteWeightCalConsts struct {
		DurationLg float64 `yaml:"durationLg"`
		AutoStake  float64 `yaml:"autoStake"`
		SelfStake  float64 `yaml:"selfStake"`
	}
	// RewardPortionCfg is contains the configs for rewarding portion contract
	RewardPortionCfg struct {
		RewardPortionContract             string `yaml:"rewardPortionContract"`
		RewardPortionContractDeployHeight uint64 `yaml:"rewardportionContractDeployHeight"`
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

// GetGravityChainStartHeight get gravity chain start height
func GetGravityChainStartHeight(
	chainClient iotexapi.APIServiceClient,
	height uint64,
) (uint64, error) {
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(PollProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(height, 10))},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return uint64(0), errors.Wrap(err, "failed to get gravity chain start height")
	}
	gravityChainStartHeight, err := strconv.ParseUint(string(readStateRes.GetData()), 10, 64)
	if err != nil {
		return uint64(0), errors.Wrap(err, "failed to parse gravityChainStartHeight")
	}
	return gravityChainStartHeight, nil
}

// GetAllStakingBuckets get all buckets by height
func GetAllStakingBuckets(chainClient iotexapi.APIServiceClient, height uint64) (voteBucketListAll *iotextypes.VoteBucketList, err error) {
	voteBucketListAll = &iotextypes.VoteBucketList{}
	for i := uint32(0); ; i++ {
		offset := i * readBucketsLimit
		size := uint32(readBucketsLimit)
		voteBucketList, err := getStakingBuckets(chainClient, offset, size, height)
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

// getStakingBuckets get specific buckets by height
func getStakingBuckets(chainClient iotexapi.APIServiceClient, offset, limit uint32, height uint64) (voteBucketList *iotextypes.VoteBucketList, err error) {
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
		Arguments:  [][]byte{arg},
		Height:     fmt.Sprintf("%d", height),
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return
	}
	voteBucketList = &iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(readStateRes.GetData(), voteBucketList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal VoteBucketList")
	}
	return
}

// GetAllStakingCandidates get all candidates by height
func GetAllStakingCandidates(chainClient iotexapi.APIServiceClient, height uint64) (candidateListAll *iotextypes.CandidateListV2, err error) {
	candidateListAll = &iotextypes.CandidateListV2{}
	for i := uint32(0); ; i++ {
		offset := i * readCandidatesLimit
		size := uint32(readCandidatesLimit)
		candidateList, err := getStakingCandidates(chainClient, offset, size, height)
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

// getStakingCandidates get specific candidates by height
func getStakingCandidates(chainClient iotexapi.APIServiceClient, offset, limit uint32, height uint64) (candidateList *iotextypes.CandidateListV2, err error) {
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
		Arguments:  [][]byte{arg},
		Height:     fmt.Sprintf("%d", height),
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return
	}
	candidateList = &iotextypes.CandidateListV2{}
	if err := proto.Unmarshal(readStateRes.GetData(), candidateList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal VoteBucketList")
	}
	return
}

// EncodeDelegateName converts a delegate name input to an internal format
func EncodeDelegateName(name string) (string, error) {
	l := len(name)
	switch {
	case l == 24:
		return name, nil
	case l <= 12:
		prefixZeros := []byte{}
		for i := 0; i < 12-len(name); i++ {
			prefixZeros = append(prefixZeros, byte(0))
		}
		suffixZeros := []byte{}
		for strings.HasSuffix(name, "#") {
			name = strings.TrimSuffix(name, "#")
			suffixZeros = append(suffixZeros, byte(0))
		}
		return hex.EncodeToString(append(append(prefixZeros, []byte(name)...), suffixZeros...)), nil
	}
	return "", errors.Errorf("invalid length %d", l)
}

// DecodeDelegateName converts format to readable delegate name
func DecodeDelegateName(name string) (string, error) {
	suffix := ""
	for strings.HasSuffix(name, "00") {
		name = strings.TrimSuffix(name, "00")
		suffix += "#"
	}
	aliasBytes, err := hex.DecodeString(strings.TrimLeft(name, "0"))
	if err != nil {
		return "", err
	}
	aliasString := string(aliasBytes) + suffix
	return aliasString, nil
}
