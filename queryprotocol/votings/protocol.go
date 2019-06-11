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
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
}

// VotingInfo defines voting infos
type VotingInfo struct {
	EpochNumber   uint64
	VoterAddress  string
	WeightedVotes string
}

// NumberOfCandidates defines number of candidates
type NumberOfCandidates struct {
	TotalCandidates    int
	ConsensusDelegates int
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// GetVotingInformation gets voting infos
func (p *Protocol) GetVotingInformation(epochNum int, delegateName string) (votingInfos []*VotingInfo, err error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		err = errors.New("votings protocol is unregistered")
		return
	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf("SELECT epoch_number, voter_address, weighted_votes FROM %s WHERE epoch_number = ? and candidate_name = ?",
		votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNum, delegateName)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}

	var votingHistory VotingInfo
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		err = errors.Wrap(err, "failed to parse results")
		return
	}

	if len(parsedRows) == 0 {
		err = indexprotocol.ErrNotExist
		return
	}

	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*VotingInfo)
		votingInfos = append(votingInfos, voting)
	}
	return
}

// GetNumberOfCandidates gets NumberOfCandidates infos
func (p *Protocol) GetNumberOfCandidates(epochNumber uint64) (numberOfCandidates *NumberOfCandidates, err error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		err = errors.New("votings protocol is unregistered")
		return
	}
	db := p.indexer.Store.GetDB()
	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get current epoch")
		return
	}
	if epochNumber > currentEpoch {
		err = errors.New("epoch number should not be greater than current epoch")
		return
	}
	getQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s where epoch_number=?", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	var totalCandidates int
	if err = stmt.QueryRow(epochNumber).Scan(&totalCandidates); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	numberOfCandidates = &NumberOfCandidates{
		totalCandidates,
		int(p.indexer.Config.NumCandidateDelegates),
	}
	return
}

// GetNumberOfWeightedVotes gets number of weighted votes
func (p *Protocol) GetNumberOfWeightedVotes(epochNumber uint64) (numberOfWeightedVotes string, err error) {
	db := p.indexer.Store.GetDB()

	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get current epoch")
		return
	}
	if epochNumber > currentEpoch {
		err = errors.New("epoch number should not be greater than current epoch")
		return
	}

	getQuery := fmt.Sprintf("SELECT SUM(total_weighted_votes) FROM %s WHERE epoch_number=?", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	defer stmt.Close()

	if err = stmt.QueryRow(epochNumber).Scan(&numberOfWeightedVotes); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return
}
