// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	// ProtocolID is the ID of protocol
	ProtocolID = "voting"
	// VotingHistoryTableName is the table name of voting history
	VotingHistoryTableName = "voting_history"
	// VotingResultTableName is the table name of voting result
	VotingResultTableName = "voting_result"
)

type (
	// VotingHistory defines the schema of "voting history" table
	VotingHistory struct {
		EpochNumber       uint64
		CandidateName     string
		VoterAddress      string
		Votes             string
		WeightedVotes     string
		RemainingDuration string
	}

	// VotingResult defines the schema of "voting result" table
	VotingResult struct {
		EpochNumber        uint64
		DelegateName       string
		OperatorAddress    string
		RewardAddress      string
		TotalWeightedVotes string
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store        s.Store
	NumDelegates uint64
	NumSubEpochs uint64
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{Store: store, NumDelegates: numDelegates, NumSubEpochs: numSubEpochs}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create voting history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] DECIMAL(65, 0) NOT NULL, [candidate_name] TEXT NOT NULL, [voter_address] VARCHAR(40) NOT NULL, [votes] DECIMAL(65, 0) NOT NULL, "+
		"[weighted_votes] DECIMAL(65, 0) NOT NULL, [remaining_duration] TEXT NOT NULL)",
		VotingHistoryTableName)); err != nil {
		return err
	}

	// create voting result table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] DECIMAL(65, 0) NOT NULL, [delegate_name] TEXT NOT NULL, [operator_address] VARCHAR(41) NOT NULL, "+
		"[reward_address] VARCHAR(41) NOT NULL, [total_weighted_votes] DECIMAL(65, 0) NOT NULL)",
		VotingResultTableName)); err != nil {
		return err
	}

	return nil
}

// Initialize initializes votings protocol
func (p *Protocol) Initialize(context.Context, *sql.Tx, *indexprotocol.Genesis) error {
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	height := blk.Height()
	epochNumber := indexprotocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, height)

	if height == indexprotocol.GetEpochHeight(epochNumber, p.NumDelegates, p.NumSubEpochs) {
		indexCtx := indexcontext.MustGetIndexCtx(ctx)
		chainClient := indexCtx.ChainClient
		electionClient := indexCtx.ElectionClient

		candidates, gravityHeight, err := p.getCandidates(chainClient, electionClient, height)
		if err != nil {
			return errors.Wrapf(err, "failed to get candidates from election service in epoch %d", epochNumber)
		}

		for _, candidate := range candidates {
			getBucketsRequest := &api.GetBucketsByCandidateRequest{
				Name:   candidate.Name,
				Height: strconv.Itoa(int(gravityHeight)),
				Offset: uint32(0),
				Limit:  math.MaxUint32,
			}
			getBucketsResponse, err := electionClient.GetBucketsByCandidate(ctx, getBucketsRequest)
			if err != nil {
				// TODO: Need a better way to handle the case that a candidate has no votes (len(voters) == 0)
				if strings.Contains(err.Error(), "offset is out of range") {
					continue
				}
				return errors.Wrapf(err, "failed to get buckets by candidate in epoch %d", epochNumber)
			}
			buckets := getBucketsResponse.Buckets
			if err := p.updateVotingHistory(tx, buckets, candidate.Name, epochNumber); err != nil {
				return errors.Wrapf(err, "failed to update voting history in epoch %d", epochNumber)
			}
			if err := p.updateVotingResult(tx, candidate, epochNumber); err != nil {
				return errors.Wrapf(err, "failed to update voting result in epoch %d", epochNumber)
			}
		}
	}
	return nil
}

// getVotingHistory gets voting history
func (p *Protocol) getVotingHistory(epochNumber uint64, candidateName string) ([]*VotingHistory, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND candidate_name=?",
		VotingHistoryTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(epochNumber, candidateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var votingHistory VotingHistory
	parsedRows, err := s.ParseSQLRows(rows, &votingHistory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	var votingHistoryList []*VotingHistory
	for _, parsedRow := range parsedRows {
		voting := parsedRow.(*VotingHistory)
		votingHistoryList = append(votingHistoryList, voting)
	}
	return votingHistoryList, nil
}

// getVotingResult gets voting result
func (p *Protocol) getVotingResult(epochNumber uint64, delegateName string) (*VotingResult, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=? AND delegate_name=?",
		VotingResultTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(epochNumber, delegateName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var votingResult VotingResult
	parsedRows, err := s.ParseSQLRows(rows, &votingResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("only one row is expected")
	}

	return parsedRows[0].(*VotingResult), nil
}

func (p *Protocol) getCandidates(
	chainClient iotexapi.APIServiceClient,
	electionClient api.APIServiceClient,
	height uint64,
) ([]*api.Candidate, uint64, error) {
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(height)},
	}
	readStateRes, err := chainClient.ReadState(context.Background(), readStateRequest)
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to get gravity chain start height")
	}
	gravityChainStartHeight := byteutil.BytesToUint64(readStateRes.Data)

	getCandidatesRequest := &api.GetCandidatesRequest{
		Height: strconv.Itoa(int(gravityChainStartHeight)),
		Offset: uint32(0),
		Limit:  math.MaxUint32,
	}

	getCandidatesResponse, err := electionClient.GetCandidates(context.Background(), getCandidatesRequest)
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to get candidates from election service")
	}
	return getCandidatesResponse.Candidates, gravityChainStartHeight, nil
}

func (p *Protocol) updateVotingHistory(tx *sql.Tx, buckets []*api.Bucket, candidateName string, epochNumber uint64) error {
	for _, bucket := range buckets {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number,candidate_name,voter_address,votes,weighted_votes,"+
			"remaining_duration) VALUES (?, ?, ?, ?, ?, ?)", VotingHistoryTableName)
		if _, err := tx.Exec(insertQuery, epochNumber, candidateName, bucket.Voter, bucket.Votes, bucket.WeightedVotes, bucket.RemainingDuration); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) updateVotingResult(tx *sql.Tx, candidate *api.Candidate, epochNumber uint64) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number,delegate_name,operator_address,reward_address,"+
		"total_weighted_votes) VALUES (?, ?, ?, ?, ?)", VotingResultTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, candidate.Name, candidate.OperatorAddress, candidate.RewardAddress, candidate.TotalWeightedVotes); err != nil {
		return err
	}
	return nil
}
