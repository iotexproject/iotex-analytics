// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-election/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// ProbationListTableName is the table name of probation list
	ProbationListTableName = "probation_list"
	// EpochAddressIndexName is the index name of epoch number and address on probation table
	EpochAddressIndexName = "epoch_address_index"
	createProbationList   = "CREATE TABLE IF NOT EXISTS %s " +
		"(epoch_number DECIMAL(65, 0) NOT NULL,intensity_rate DECIMAL(65, 0) NOT NULL,address VARCHAR(41) NOT NULL, count DECIMAL(65, 0) NOT NULL,PRIMARY KEY (`epoch_number`, `address`), UNIQUE KEY %s (epoch_number, address))"
	insertProbationList = "INSERT IGNORE INTO %s (epoch_number,intensity_rate,address,count) VALUES (?, ?, ?, ?)"
	selectProbationList = "SELECT * FROM %s WHERE epoch_number=?"
)

type (
	// ProbationList defines the schema of "probation_list" table
	ProbationList struct {
		EpochNumber   uint64
		IntensityRate uint64
		Address       string
		Count         uint64
	}
)

func (p *Protocol) createProbationListTable(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf(createProbationList, ProbationListTableName, EpochAddressIndexName)); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) updateProbationListTable(tx *sql.Tx, epochNum uint64, probationList *iotextypes.ProbationCandidateList) error {
	if probationList == nil {
		return nil
	}
	insertQuery := fmt.Sprintf(insertProbationList, ProbationListTableName)
	for _, k := range probationList.ProbationList {
		if _, err := tx.Exec(insertQuery, epochNum, probationList.IntensityRate, k.Address, k.Count); err != nil {
			return errors.Wrap(err, "failed to update probation list table")
		}
	}
	return nil
}

func (p *Protocol) fetchProbationList(cli iotexapi.APIServiceClient, epochNum uint64) (*iotextypes.ProbationCandidateList, error) {
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ProbationListByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epochNum, 10))},
	}
	out, err := cli.ReadState(context.Background(), request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	probationList := &iotextypes.ProbationCandidateList{}
	if out.Data != nil {
		if err := proto.Unmarshal(out.Data, probationList); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal probationList")
		}
	}
	return probationList, nil
}

// getProbationList gets probation list from table
func (p *Protocol) getProbationList(epochNumber uint64) ([]*ProbationList, error) {
	db := p.Store.GetDB()
	getQuery := fmt.Sprintf(selectProbationList,
		ProbationListTableName)
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()
	rows, err := stmt.Query(epochNumber)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}
	var pb ProbationList
	parsedRows, err := s.ParseSQLRows(rows, &pb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}
	if len(parsedRows) == 0 {
		return nil, nil
	}
	var pblist []*ProbationList
	for _, parsedRow := range parsedRows {
		pb := parsedRow.(*ProbationList)
		pblist = append(pblist, pb)
	}
	return pblist, nil
}

// filterCandidates returns filtered candidate list by given raw candidate and probation list
func filterCandidates(
	candidates []*types.Candidate,
	unqualifiedList *iotextypes.ProbationCandidateList,
	epochStartHeight uint64,
) ([]*types.Candidate, error) {
	candidatesMap := make(map[string]*types.Candidate)
	updatedVotingPower := make(map[string]*big.Int)
	intensityRate := float64(uint32(100)-unqualifiedList.IntensityRate) / float64(100)

	probationMap := make(map[string]uint32)
	for _, elem := range unqualifiedList.ProbationList {
		probationMap[elem.Address] = elem.Count
	}
	for _, cand := range candidates {
		filterCand := cand.Clone()
		candOpAddr := string(cand.OperatorAddress())
		if _, ok := probationMap[candOpAddr]; ok {
			// if it is an unqualified delegate, multiply the voting power with probation intensity rate
			votingPower := new(big.Float).SetInt(filterCand.Score())
			newVotingPower, _ := votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
			filterCand.SetScore(newVotingPower)
		}
		updatedVotingPower[candOpAddr] = filterCand.Score()
		candidatesMap[candOpAddr] = filterCand
	}
	// sort again with updated voting power
	sorted := util.Sort(updatedVotingPower, epochStartHeight)
	var verifiedCandidates []*types.Candidate
	for _, name := range sorted {
		verifiedCandidates = append(verifiedCandidates, candidatesMap[name])
	}
	return verifiedCandidates, nil
}

// filterStakingCandidates returns filtered candidate list by given raw candidate and probation list
func filterStakingCandidates(
	candidates *iotextypes.CandidateListV2,
	unqualifiedList *iotextypes.ProbationCandidateList,
	epochStartHeight uint64,
) (*iotextypes.CandidateListV2, error) {
	candidatesMap := make(map[string]*iotextypes.CandidateV2)
	updatedVotingPower := make(map[string]*big.Int)
	intensityRate := float64(uint32(100)-unqualifiedList.IntensityRate) / float64(100)

	probationMap := make(map[string]uint32)
	for _, elem := range unqualifiedList.ProbationList {
		probationMap[elem.Address] = elem.Count
	}
	for _, cand := range candidates.Candidates {
		filterCand := *cand
		votingPower, ok := new(big.Float).SetString(cand.TotalWeightedVotes)
		if !ok {
			return nil, errors.New("total weighted votes convert error")
		}
		if _, ok := probationMap[cand.OperatorAddress]; ok {
			newVotingPower, _ := votingPower.Mul(votingPower, big.NewFloat(intensityRate)).Int(nil)
			filterCand.TotalWeightedVotes = newVotingPower.String()
		}
		totalWeightedVotes, ok := new(big.Int).SetString(filterCand.TotalWeightedVotes, 10)
		if !ok {
			return nil, errors.New("total weighted votes convert error")
		}
		updatedVotingPower[cand.OperatorAddress] = totalWeightedVotes
		candidatesMap[cand.OperatorAddress] = &filterCand
	}
	// sort again with updated voting power
	sorted := util.Sort(updatedVotingPower, epochStartHeight)
	verifiedCandidates := &iotextypes.CandidateListV2{}
	for _, name := range sorted {
		verifiedCandidates.Candidates = append(verifiedCandidates.Candidates, candidatesMap[name])
	}
	return verifiedCandidates, nil
}

func stakingProbationListToMap(candidateList *iotextypes.CandidateListV2, probationList []*ProbationList) (intensityRate float64, probationMap map[string]uint64) {
	probationMap = make(map[string]uint64)
	if probationList != nil {
		for _, can := range candidateList.Candidates {
			for _, pb := range probationList {
				intensityRate = float64(uint64(100)-pb.IntensityRate) / float64(100)
				if pb.Address == can.OperatorAddress {
					probationMap[can.OwnerAddress] = pb.Count
				}
			}
		}
	}
	return
}

func probationListToMap(delegates []*types.Candidate, pblist []*ProbationList) (intensityRate float64, probationMap map[string]uint64) {
	probationMap = make(map[string]uint64)
	if pblist != nil {
		for _, delegate := range delegates {
			delegateOpAddr := string(delegate.OperatorAddress())
			for _, pb := range pblist {
				intensityRate = float64(uint64(100)-pb.IntensityRate) / float64(100)
				if pb.Address == delegateOpAddr {
					probationMap[hex.EncodeToString(delegate.Name())] = pb.Count
				}
			}
		}
	}
	return
}

func convertProbationListToLocal(probationList *iotextypes.ProbationCandidateList) (ret []*ProbationList) {
	if probationList == nil {
		return nil
	}
	ret = make([]*ProbationList, 0)
	for _, pb := range probationList.ProbationList {
		p := &ProbationList{
			0,
			uint64(probationList.IntensityRate),
			pb.Address,
			uint64(pb.Count),
		}
		ret = append(ret, p)
	}
	return
}
