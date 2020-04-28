// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	candidateCreationSQLITE = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT UNIQUE, owner BLOB, operator BLOB, reward BLOB, name BLOB, votes BLOB, self_stake_bucket_idx DECIMAL(65, 0), self_stake BLOB)"
	candidateCreationMySQL  = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTO_INCREMENT, hash VARCHAR(64) UNIQUE, owner BLOB, operator BLOB, reward BLOB, name BLOB, votes BLOB, self_stake_bucket_idx DECIMAL(65, 0), self_stake BLOB)"

	// InsertCandidateQuerySQLITE is query to insert candidates in SQLITE driver
	InsertCandidateQuerySQLITE = "INSERT OR IGNORE INTO %s (hash, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	// InsertCandidateQueryMySQL is query to insert candidates in MySQL driver
	InsertCandidateQueryMySQL = "INSERT IGNORE INTO %s (hash, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	// CandidateQuery is query to get candidates by ids
	CandidateQuery = "SELECT id, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake FROM %s WHERE id IN (%s)"
)

type (
	candidateStruct struct {
		Id                                   int64
		Owner, Operator, Reward, Name, Votes []byte
		Self_stake_bucket_idx                uint64
		Self_stake                           []byte
	}
)

// NewCandidateTableOperator create an operator for candidate table
func NewCandidateTableOperator(tableName string, driverName committee.DRIVERTYPE) (committee.Operator, error) {
	var creation string
	switch driverName {
	case committee.SQLITE:
		creation = candidateCreationSQLITE
	case committee.MYSQL:
		creation = candidateCreationMySQL
	default:
		return nil, errors.New("Wrong driver type")
	}
	return committee.NewRecordTableOperator(
		tableName,
		driverName,
		InsertCandidates,
		QueryCandidates,
		creation,
	)
}

// QueryCandidates get all candidates by ids
func QueryCandidates(tableName string, frequencies map[int64]int, sdb *sql.DB, tx *sql.Tx) (ret interface{}, err error) {
	size := 0
	ids := make([]int64, 0, len(frequencies))
	for id, f := range frequencies {
		ids = append(ids, id)
		size += f
	}
	var rows *sql.Rows
	if tx != nil {
		rows, err = tx.Query(fmt.Sprintf(CandidateQuery, tableName, atos(ids)))
	} else {
		rows, err = sdb.Query(fmt.Sprintf(CandidateQuery, tableName, atos(ids)))
	}
	if err != nil {
		return
	}
	if err = rows.Err(); err != nil {
		return
	}
	defer rows.Close()
	var cs candidateStruct
	parsedRows, err := s.ParseSQLRows(rows, &cs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	candidates := make([]*iotextypes.CandidateV2, 0)
	for _, parsedRow := range parsedRows {
		cs, ok := parsedRow.(*candidateStruct)
		if !ok {
			return nil, errors.New("failed to convert")
		}
		candidate := &iotextypes.CandidateV2{
			OwnerAddress:       string(cs.Owner),
			OperatorAddress:    string(cs.Operator),
			RewardAddress:      string(cs.Reward),
			Name:               string(cs.Name),
			TotalWeightedVotes: string(cs.Votes),
			SelfStakeBucketIdx: cs.Self_stake_bucket_idx,
			SelfStakingTokens:  string(cs.Self_stake),
		}
		for i := frequencies[cs.Id]; i > 0; i-- {
			candidates = append(candidates, candidate)
		}
	}

	return &iotextypes.CandidateListV2{Candidates: candidates}, nil
}

// InsertCandidates inserts candidate records into table by tx
func InsertCandidates(tableName string, driverName committee.DRIVERTYPE, records interface{}, tx *sql.Tx) (frequencies map[hash.Hash256]int, err error) {
	candidates, ok := records.(*iotextypes.CandidateListV2)
	if !ok {
		return nil, errors.Errorf("Unexpected type %s", reflect.TypeOf(records))
	}
	if candidates == nil {
		return nil, nil
	}
	var candStmt *sql.Stmt
	switch driverName {
	case committee.SQLITE:
		candStmt, err = tx.Prepare(fmt.Sprintf(InsertCandidateQuerySQLITE, tableName))
	case committee.MYSQL:
		candStmt, err = tx.Prepare(fmt.Sprintf(InsertCandidateQueryMySQL, tableName))
	default:
		return nil, errors.New("wrong driver type")
	}
	defer func() {
		closeErr := candStmt.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	frequencies = make(map[hash.Hash256]int)
	for _, candidate := range candidates.Candidates {
		var h hash.Hash256
		if h, err = hashCandidate(candidate); err != nil {
			return nil, err
		}
		if f, ok := frequencies[h]; ok {
			frequencies[h] = f + 1
		} else {
			frequencies[h] = 1
		}
		if _, err = candStmt.Exec(
			hex.EncodeToString(h[:]),
			[]byte(candidate.OwnerAddress),
			[]byte(candidate.OperatorAddress),
			[]byte(candidate.RewardAddress),
			[]byte(candidate.Name),
			[]byte(candidate.TotalWeightedVotes),
			candidate.SelfStakeBucketIdx,
			[]byte(candidate.SelfStakingTokens),
		); err != nil {
			return nil, err
		}
	}

	return frequencies, nil
}

func hashCandidate(candidate *iotextypes.CandidateV2) (hash.Hash256, error) {
	data, err := proto.Marshal(candidate)
	if err != nil {
		return hash.ZeroHash256, err
	}
	return hash.Hash256b(data), nil
}
