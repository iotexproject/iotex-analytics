// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"fmt"
	"strconv"

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
	getQuery := fmt.Sprintf("SELECT voter_address,weighted_votes FROM %s WHERE epoch_number = ? and candidate_name = ?",
		votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}

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

//GetTotalTokenStaked gets TotalTokenStaked infos
func (p *Protocol) GetTotalTokenStaked(startEpoch uint64, epochCount uint64) ([]string, error) {
	var totalTokenStaked []string
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		err := errors.New("votings protocol is unregistered")
		return nil, err
	}
	db := p.indexer.Store.GetDB()
	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get current epoch")
		return nil, err
	}
	if startEpoch+epochCount > currentEpoch {
		err = errors.New("epoch number should not be greater than current epoch")
		return totalTokenStaked, err
	}
	getQuery := fmt.Sprintf("SELECT SUM(votes) FROM %s WHERE epoch_number>=? and epoch_number <= ? GROUP BY epoch_number", votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return nil, err
	}
	rows, err := stmt.Query(startEpoch, startEpoch+epochCount)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return nil, err
	}
	for rows.Next() {
		var r string
		err = rows.Scan(&r)
		if err != nil {
			return nil, err
		}
		totalTokenStaked = append(totalTokenStaked, r)
	}
	return totalTokenStaked, nil

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
func (p *Protocol) GetNumberOfWeightedVotes(startEpoch uint64, epochCount uint64) (numberOfWeightedVotes []string, err error) {
	db := p.indexer.Store.GetDB()

	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		err = errors.Wrap(err, "failed to get current epoch")
		return
	}
	if startEpoch+epochCount > currentEpoch {
		err = errors.New("epoch number should not be greater than current epoch")
		return
	}

	getQuery := fmt.Sprintf("SELECT SUM(total_weighted_votes) FROM %s WHERE epoch_number>=? and epoch_number<=? GROUP BY epoch_number", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	rows, err := stmt.Query(startEpoch, startEpoch+epochCount)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return nil, err
	}

	for rows.Next() {
		var r string
		err = rows.Scan(&r)
		if err != nil {
			return nil, err
		}
		numberOfWeightedVotes = append(numberOfWeightedVotes, r)
	}
	return
}

// GetStakeDuration gets array of stake duration in specific epoch
func (p *Protocol) GetStakeDuration(epochNumber uint64) (stakeDuration []string, err error) {
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

	getQuery := fmt.Sprintf("SELECT remaining_duration FROM %s WHERE epoch_number=?", votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	rows, err := stmt.Query(epochNumber)
	if err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return nil, err
	}

	for rows.Next() {
		var r string
		err = rows.Scan(&r)
		if err != nil {
			return nil, err
		}
		stakeDuration = append(stakeDuration, r)
	}
	return

}

// GetNumberOfUniqueAccounts gets array of stake duration in specific epoch
func (p *Protocol) GetNumberOfUniqueAccounts(epochNumber uint64, delegateName string) (numberUniqueAccounts string, err error) {
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

	getQuery := fmt.Sprintf("SELECT count(DISTINCT voter_address) FROM %s WHERE candidate_name=? and epoch_number=?", votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	if err = stmt.QueryRow(delegateName, epochNumber).Scan(&numberUniqueAccounts); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	return

}

// GetAverageTokenHolding gets average token holding
func (p *Protocol) GetAverageTokenHolding(epochNumber uint64) (numberAverageHolding string, err error) {
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

	getQueryAccount := fmt.Sprintf("SELECT count(DISTINCT voter_address) FROM %s WHERE epoch_number=?", votings.VotingHistoryTableName)
	stmt, err := db.Prepare(getQueryAccount)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var accountNumber string
	if err = stmt.QueryRow(epochNumber).Scan(&accountNumber); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	getQuery := fmt.Sprintf("SELECT SUM(votes) FROM %s WHERE epoch_number=?", votings.VotingHistoryTableName)
	stmt, err = db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var totalTokens string
	if err = stmt.QueryRow(epochNumber).Scan(&totalTokens); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	accounts, err := strconv.ParseFloat(accountNumber, 64)
	tokens, err := strconv.ParseFloat(totalTokens, 64)
	if err != nil {
		err = errors.Wrap(err, "failed transform float")
		return
	}
	numberAverageHolding = fmt.Sprintf("%f", tokens/accounts)
	return

}

// GetSelfStakeAndTotalStake gets for specific epoch and delegate return its selfstake vote and total vote
func (p *Protocol) GetSelfStakeAndTotalStake(epochNumber uint64, delegateName string) (votes []string, err error) {
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
	getQuerySelf := fmt.Sprintf("SELECT self_total_votes FROM %s WHERE epoch_number=?", votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuerySelf)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var self string
	if err = stmt.QueryRow(epochNumber).Scan(&self); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	getQuery := fmt.Sprintf("SELECT total_weighted_votes FROM %s WHERE epoch_number=?", votings.VotingResultTableName)
	stmt, err = db.Prepare(getQuery)
	if err != nil {
		err = errors.Wrap(err, "failed to prepare get query")
		return
	}
	var total string
	if err = stmt.QueryRow(epochNumber).Scan(&total); err != nil {
		err = errors.Wrap(err, "failed to execute get query")
		return
	}
	votes = append(votes, self)
	votes = append(votes, total)
	return

}
