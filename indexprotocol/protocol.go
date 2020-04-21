// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexprotocol

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

// PollProtocolID is ID of poll protocol
const PollProtocolID = "poll"

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
)

// Protocol defines the protocol interfaces for block indexer
type Protocol interface {
	BlockHandler
	CreateTables(context.Context) error
	Initialize(context.Context, *sql.Tx, *Genesis) error
}

// BlockHandler ishte interface of handling block
type BlockHandler interface {
	HandleBlock(context.Context, *sql.Tx, *block.Block) error
}
