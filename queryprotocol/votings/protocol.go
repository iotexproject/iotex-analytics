// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/votings"
	"github.com/iotexproject/iotex-analytics/indexservice"
	"github.com/iotexproject/iotex-analytics/queryprotocol"
	"github.com/iotexproject/iotex-analytics/queryprotocol/chainmeta/chainmetautil"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	selectVotingResultWithName = "SELECT epoch_number,total_weighted_votes,self_staking FROM %s WHERE epoch_number >= ? AND epoch_number <= ? AND delegate_name = ?"
	selectVotingResult         = "SELECT delegate_name, staking_address, total_weighted_votes, self_staking, operator_address, reward_address FROM %s WHERE epoch_number = ?"
	selectVotingMeta           = "SELECT * FROM %s where epoch_number >= ? AND epoch_number <= ?"
	selectDelegate             = "SELECT delegate_name FROM %s WHERE operator_address=? ORDER BY epoch_number DESC LIMIT 1"
	selectOperator             = "SELECT operator_address FROM %s WHERE delegate_name=? ORDER BY epoch_number DESC LIMIT 1"
	selectOperatorOfEpoch      = "SELECT operator_address FROM %s WHERE delegate_name=? and epoch_number<=? order by epoch_number DESC LIMIT 1"
	selectProbationExist       = "select * from %s where epoch_number=%d and address='%s'"
	selectAppearingCount       = "select count(epoch_number) from %s where epoch_number>=%d and epoch_number<%d and delegate_name=?"
	selectBuckets              = "SELECT a.id, a.`index`, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake FROM %s AS a RIGHT JOIN (SELECT `index`, MAX(`id`) AS mid FROM %s GROUP BY `index`) AS b ON a.id = b.mid and a.index = b.index"
	selectBucketsByCandidate   = selectBuckets + " WHERE candidate = ? ORDER BY `id` LIMIT ?, ?"
	selectBucketsByVoter       = selectBuckets + " WHERE owner = ? ORDER BY `id` LIMIT ?, ?"
)

// Protocol defines the protocol of querying tables
type Protocol struct {
	indexer *indexservice.Indexer
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

//CandidateInfo defines candidate info
type CandidateInfo struct {
	Name               string
	Address            string
	TotalWeightedVotes string
	SelfStakingTokens  string
	OperatorAddress    string
	RewardAddress      string
}

// NewProtocol creates a new protocol
func NewProtocol(idx *indexservice.Indexer) *Protocol {
	return &Protocol{indexer: idx}
}

// BucketsByVoter returns the buckets owned by the voter
func (p *Protocol) BucketsByVoter(voter address.Address, offset uint64, size uint64) (map[int64]*iotextypes.VoteBucket, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")
	}
	stmt, err := p.indexer.Store.GetDB().Prepare(fmt.Sprintf(selectBucketsByVoter, votings.StakingBucketInfoTableName, votings.StakingBucketInfoTableName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare query")
	}
	defer stmt.Close()
	rows, err := stmt.Query(voter.String(), offset, size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}

	return votings.ParseBuckets(rows)
}

// BucketsByCandidate returns the buckets voted to a candidate
func (p *Protocol) BucketsByCandidate(candidate string, offset uint64, size uint64) (map[int64]*iotextypes.VoteBucket, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")
	}
	var candidateAddr string
	if err := p.indexer.Store.GetDB().QueryRow("SELECT owner FROM "+votings.StakingCandidatesTableName+" WHERE hex(name) = ?", hex.EncodeToString([]byte(candidate))).Scan(&candidateAddr); err != nil {
		return nil, errors.Wrap(err, "failed to query candidate address")
	}
	stmt, err := p.indexer.Store.GetDB().Prepare(fmt.Sprintf(selectBucketsByCandidate, votings.StakingBucketInfoTableName, votings.StakingBucketInfoTableName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare query")
	}
	defer stmt.Close()
	fmt.Println("candidate address", candidateAddr)
	rows, err := stmt.Query(string(candidateAddr), offset, size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}

	buckets, err := votings.ParseBuckets(rows)
	if err != nil {
		return nil, err
	}
	retval := map[int64]*iotextypes.VoteBucket{}
	for index, bucket := range buckets {
		if bucket.UnstakeStartTime.AsTime().After(bucket.StakeStartTime.AsTime()) {
			continue
		}
		retval[index] = bucket
	}
	return retval, nil
}

// GetBucketInformation gets voting infos
func (p *Protocol) GetBucketInformation(startEpoch uint64, epochCount uint64, delegateName string) (map[uint64][]*votings.VotingInfo, error) {
	var protocol indexprotocol.Protocol
	var votingProtocol *votings.Protocol
	var ok bool
	if protocol, ok = p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return nil, errors.New("votings protocol is unregistered")
	}
	if votingProtocol, ok = protocol.(*votings.Protocol); !ok {
		return nil, errors.New("failed to cast to voting protocol")
	}

	currentEpoch, _, err := chainmetautil.GetCurrentEpochAndHeight(p.indexer.Registry, p.indexer.Store)
	if err != nil {
		return nil, errors.New("failed to get most recent epoch")
	}
	endEpoch := startEpoch + epochCount - 1
	if endEpoch > currentEpoch {
		endEpoch = currentEpoch
	}

	bucketInfoMap := make(map[uint64][]*votings.VotingInfo)
	for i := startEpoch; i <= endEpoch; i++ {
		voteInfoList, err := votingProtocol.GetBucketInfoByEpoch(i, delegateName)
		if err != nil {
			return nil, err
		}
		bucketInfoMap[i] = voteInfoList
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

	getQuery := fmt.Sprintf(selectVotingResultWithName, votings.VotingResultTableName)
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
		return nil, indexprotocol.ErrNotExist
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

	endEpoch := startEpoch + epochCount - 1

	getQuery := fmt.Sprintf(selectVotingMeta, votings.VotingMetaTableName)
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
		return nil, 0, indexprotocol.ErrNotExist
	}

	candidateMetaList := make([]*CandidateMeta, 0)
	for _, parsedRow := range parsedRows {
		candidateMeta := parsedRow.(*CandidateMeta)
		candidateMetaList = append(candidateMetaList, candidateMeta)
	}
	return candidateMetaList, p.indexer.Config.NumCandidateDelegates, nil
}

//GetCandidates gets a list of candidate info
func (p *Protocol) GetCandidates(startEpoch uint64, epochCount uint64) (map[uint64][]*CandidateInfo, error) {
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

	candidateInfoMap := make(map[uint64][]*CandidateInfo)
	for i := startEpoch; i <= endEpoch; i++ {
		voteInfoList, err := p.getCandidateInfoByEpoch(i)
		if err != nil {
			return nil, err
		}
		candidateInfoMap[i] = voteInfoList
	}
	return candidateInfoMap, nil
}

func (p *Protocol) getCandidateInfoByEpoch(epochNumber uint64) ([]*CandidateInfo, error) {
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectVotingResult, votings.VotingResultTableName)

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(epochNumber)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var candidateInfo CandidateInfo
	parsedRows, err := s.ParseSQLRows(rows, &candidateInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}
	candidateInfoList := make([]*CandidateInfo, 0)
	for _, parsedRow := range parsedRows {
		candidateInfo := parsedRow.(*CandidateInfo)
		candidateInfoList = append(candidateInfoList, candidateInfo)
	}
	return candidateInfoList, nil
}

//GetAlias gets operator name
func (p *Protocol) GetAlias(operatorAddress string) (string, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return "", errors.New("votings protocol is unregistered")

	}
	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectDelegate,
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	var name string
	if err = stmt.QueryRow(operatorAddress).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", indexprotocol.ErrNotExist
		}
		return "", errors.Wrap(err, "failed to execute get query")
	}

	return name, nil
}

//GetOperatorAddress gets operator name
func (p *Protocol) GetOperatorAddress(aliasName string) (string, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return "", errors.New("votings protocol is unregistered")

	}
	db := p.indexer.Store.GetDB()

	getQuery := fmt.Sprintf(selectOperator,
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	var address string
	if err = stmt.QueryRow(aliasName).Scan(&address); err != nil {
		if err == sql.ErrNoRows {
			return "", indexprotocol.ErrNotExist
		}
		return "", errors.Wrap(err, "failed to execute get query")
	}

	return address, nil
}

//GetProbationHistoricalRate gets probation rate
func (p *Protocol) GetProbationHistoricalRate(startEpoch int, epochCount int, delegateName string) (string, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return "", errors.New("votings protocol is unregistered")
	}
	db := p.indexer.Store.GetDB()
	appearingCount, err := p.getAppearingCount(db, startEpoch, epochCount, delegateName)
	if err != nil {
		return "0", errors.New("get Appearing Count error")
	}
	if appearingCount == 0 {
		return "0", nil
	}
	probationCount := uint64(0)
	for i := startEpoch; i < startEpoch+epochCount; i++ {
		address, err := p.getOperatorAddress(delegateName, i)
		switch {
		case errors.Cause(err) == indexprotocol.ErrNotExist:
			continue
		case err != nil:
			return "0", err
		}
		exist, _ := queryprotocol.RowExists(db, fmt.Sprintf(selectProbationExist,
			votings.ProbationListTableName, i, address))
		if exist {
			probationCount++
		}
	}
	rate := float64(probationCount) / float64(appearingCount)
	return fmt.Sprintf("%0.2f", rate), nil
}

func (p *Protocol) getAppearingCount(db *sql.DB, startEpoch int, epochCount int, delegateName string) (count uint64, err error) {
	getQuery := fmt.Sprintf(selectAppearingCount,
		votings.VotingResultTableName, startEpoch, startEpoch+epochCount)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return
	}
	defer stmt.Close()
	if err = stmt.QueryRow(delegateName).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to execute get query")
	}
	return
}

func (p *Protocol) getOperatorAddress(delegateName string, epoch int) (string, error) {
	if _, ok := p.indexer.Registry.Find(votings.ProtocolID); !ok {
		return "", errors.New("votings protocol is unregistered")

	}
	db := p.indexer.Store.GetDB()
	getQuery := fmt.Sprintf(selectOperatorOfEpoch,
		votings.VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	var address string
	if err = stmt.QueryRow(delegateName, epoch).Scan(&address); err != nil {
		if err == sql.ErrNoRows {
			return "", indexprotocol.ErrNotExist
		}
		return "", errors.Wrap(err, "failed to execute get query")
	}

	return address, nil
}
