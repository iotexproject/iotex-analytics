// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// VotingInfo defines voting info
type VotingInfo struct {
	EpochNumber   uint64
	VoterAddress  string
	WeightedVotes string
}

// CandidateMeta defines candidate mata data
type CandidateMeta struct {
	EpochNumber        uint64
	VotedTokens        string
	NumberOfCandidates uint64
	TotalWeightedVotes string
}

//StakingInfo defines staked information
type StakingInfo struct {
	EpochNumber  uint64
	TotalStaking string
	SelfStaking  string
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetBucketInformation gets voting infos
func (p *Protocol) GetBucketInformation(startEpoch uint64, epochCount uint64, delegateName string) (map[uint64][]*VotingInfo, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")
	}

	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		return nil, errors.New("failed to get most recent epoch")
	}
	endEpoch := startEpoch + epochCount - 1
	if endEpoch > currentEpoch {
		endEpoch = currentEpoch
	}

	db := p.indexer.Store.GetDB()

	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number >= ? and epoch_number <= ? and candidate_name = ?",
		votings.VotingHistoryTableName), startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return nil, indexprotocol.ErrNotExist
	}

	getQuery := fmt.Sprintf("SELECT epoch_number, voter_address, weighted_votes FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND candidate_name = ?",
		votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var votingHistory VotingInfo
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	bucketInfoMap := make(map[uint64][]*VotingInfo)
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*VotingInfo)
		if _, ok := bucketInfoMap[voting.EpochNumber]; !ok {
			bucketInfoMap[voting.EpochNumber] = make([]*VotingInfo, 0)
		}
		bucketInfoMap[voting.EpochNumber] = append(bucketInfoMap[voting.EpochNumber], voting)
	}
	return bucketInfoMap, nil
}

//GetStaking get staked information
func (p *Protocol) GetStaking(startEpoch uint64, epochCount uint64, delegateName string) ([]*StakingInfo, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")

	}
	db := p.indexer.Store.GetDB()

	endEpoch := startEpoch + epochCount - 1

	// Check existence
	exist, err := queryprotocol.RowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE epoch_number >= ? and epoch_number <= ? and delegate_name = ?",
		votings.VotingResultTableName), startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if the row exists")
	}
	if !exist {
		return nil, indexprotocol.ErrNotExist
	}

	getQuery := fmt.Sprintf("SELECT epoch_number,total_weighted_votes,self_staking FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND delegate_name = ?", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var stakingInfo StakingInfo
	parsedRows, err := s.ParseSQLRows(rows, &stakingInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, errors.Wrapf(err, "missing data in %s", votings.VotingResultTableName)
	}
	stakingInfoList := make([]*StakingInfo, 0)
	for _, parsedRow := range parsedRows {
		stakingInfo := parsedRow.(*StakingInfo)
		stakingInfoList = append(stakingInfoList, stakingInfo)
	}
	return stakingInfoList, nil
}

// GetCandidateMeta gets candidate metadata
func (p *Protocol) GetCandidateMeta(startEpoch uint64, epochCount uint64) ([]*CandidateMeta, uint64, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, 0, errors.New("votings protocol is unregistered")

	}
	db := p.indexer.Store.GetDB()
	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get current epoch")
	}
	if startEpoch > currentEpoch {
		return nil, 0, indexprotocol.ErrNotExist
	}

	endEpoch := startEpoch + epochCount - 1

	getQuery := fmt.Sprintf("SELECT * FROM %s where epoch_number >= ? AND epoch_number <= ?", votings.VotingMetaTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(startEpoch, endEpoch)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to execute get query")
	}

	var candidateMeta CandidateMeta
	parsedRows, err := s.ParseSQLRows(rows, &candidateMeta)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, 0, errors.Wrapf(err, "missing data in %s", votings.VotingResultTableName)
	}

	candidateMetaList := make([]*CandidateMeta, 0)
	for _, parsedRow := range parsedRows {
		candidateMeta := parsedRow.(*CandidateMeta)
		candidateMetaList = append(candidateMetaList, candidateMeta)
	}
	return candidateMetaList, p.indexer.Config.NumCandidateDelegates, nil
}
