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
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
)

const (
	bucketCreationSQLITE = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT UNIQUE, index DECIMAL(65, 0), candidate BLOB, owner BLOB, staked_amount BLOB, staked_duration TEXT, create_time TIMESTAMP NULL DEFAULT NULL, stake_start_time TIMESTAMP NULL DEFAULT NULL, unstake_start_time TIMESTAMP NULL DEFAULT NULL, auto_stake INTEGER)"
	bucketCreationMySQL  = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTO_INCREMENT, hash VARCHAR(64) UNIQUE, `index` DECIMAL(65, 0), candidate BLOB, owner BLOB, staked_amount BLOB, staked_duration TEXT, create_time TIMESTAMP NULL DEFAULT NULL, stake_start_time TIMESTAMP NULL DEFAULT NULL, unstake_start_time TIMESTAMP NULL DEFAULT NULL, auto_stake INTEGER)"

	// InsertVoteBucketsQuery is query to insert vote buckets for sqlite
	InsertVoteBucketsQuery = "INSERT OR IGNORE INTO %s (hash, index, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	// InsertVoteBucketsQueryMySql is query to insert vote buckets for mysql
	InsertVoteBucketsQueryMySql = "INSERT IGNORE INTO %s (hash, `index`, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	// VoteBucketRecordQuery is query to return vote buckets by ids
	VoteBucketRecordQuery = "SELECT id, `index`, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake FROM %s WHERE id IN (%s)"
)

type (
	bucket struct {
		Id                                           int64
		Index                                        uint64
		Candidate, Owner, StakedAmount               []byte
		StakedDuration                               string
		CreateTime, StakeStartTime, UnstakeStartTime time.Time
		AutoStake                                    int64
	}
)

// NewVoteBucketTableOperator creates an operator for vote bucket table
func NewBucketTableOperator(tableName string, driverName committee.DRIVERTYPE) (committee.Operator, error) {
	var creation string
	switch driverName {
	case committee.SQLITE:
		creation = bucketCreationSQLITE
	case committee.MYSQL:
		creation = bucketCreationMySQL
	default:
		return nil, errors.New("Wrong driver type")
	}
	return committee.NewRecordTableOperator(
		tableName,
		driverName,
		InsertVoteBuckets,
		QueryVoteBuckets,
		creation,
	)
}

// QueryVoteBuckets returns vote buckets by ids
func QueryVoteBuckets(tableName string, frequencies map[int64]int, sdb *sql.DB, tx *sql.Tx) (ret interface{}, err error) {
	size := 0
	ids := make([]int64, 0, len(frequencies))
	for id, f := range frequencies {
		ids = append(ids, id)
		size += f
	}
	var rows *sql.Rows
	if tx != nil {
		rows, err = tx.Query(fmt.Sprintf(VoteBucketRecordQuery, tableName, atos(ids)))
	} else {
		rows, err = sdb.Query(fmt.Sprintf(VoteBucketRecordQuery, tableName, atos(ids)))
	}
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	defer rows.Close()
	var b bucket
	parsedRows, err := s.ParseSQLRows(rows, &b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}

	buckets := make([]*iotextypes.VoteBucket, 0)
	for _, parsedRow := range parsedRows {
		b, ok := parsedRow.(*bucket)
		if !ok {
			return nil, errors.New("failed to convert")
		}
		duration, err := strconv.ParseUint(b.StakedDuration, 10, 32)
		if err != nil {
			return nil, err
		}
		createTime, err := ptypes.TimestampProto(b.CreateTime)
		if err != nil {
			return nil, err
		}
		stakeTime, err := ptypes.TimestampProto(b.StakeStartTime)
		if err != nil {
			return nil, err
		}
		unstakeTime, err := ptypes.TimestampProto(b.UnstakeStartTime)
		if err != nil {
			return nil, err
		}
		bucket := &iotextypes.VoteBucket{
			Index:            b.Index,
			CandidateAddress: string(b.Candidate),
			Owner:            string(b.Owner),
			StakedAmount:     string(b.StakedAmount),
			StakedDuration:   uint32(duration),
			CreateTime:       createTime,
			StakeStartTime:   stakeTime,
			UnstakeStartTime: unstakeTime,
			AutoStake:        b.AutoStake == 1,
		}
		for i := frequencies[b.Id]; i > 0; i-- {
			buckets = append(buckets, bucket)
		}
	}

	return &iotextypes.VoteBucketList{Buckets: buckets}, nil
}

// InsertVoteBuckets inserts vote bucket records into table by tx
func InsertVoteBuckets(tableName string, driverName committee.DRIVERTYPE, records interface{}, tx *sql.Tx) (frequencies map[hash.Hash256]int, err error) {
	buckets, ok := records.(*iotextypes.VoteBucketList)
	if !ok {
		return nil, errors.Errorf("invalid record type %s, *types.Bucket expected", reflect.TypeOf(records))
	}
	if buckets == nil {
		return nil, nil
	}
	var stmt *sql.Stmt
	switch driverName {
	case committee.SQLITE:
		stmt, err = tx.Prepare(fmt.Sprintf(InsertVoteBucketsQuery, tableName))
	case committee.MYSQL:
		fmt.Println(fmt.Sprintf(InsertVoteBucketsQueryMySql, tableName))
		stmt, err = tx.Prepare(fmt.Sprintf(InsertVoteBucketsQueryMySql, tableName))
	default:
		return nil, errors.New("wrong driver type")
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := stmt.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	frequencies = make(map[hash.Hash256]int)
	for _, bucket := range buckets.Buckets {
		h, err := hashBucket(bucket)
		if err != nil {
			return nil, err
		}
		if f, ok := frequencies[h]; ok {
			frequencies[h] = f + 1
		} else {
			frequencies[h] = 1
		}
		duration := big.NewInt(0).SetUint64(uint64(bucket.StakedDuration))
		ct := time.Unix(bucket.CreateTime.Seconds, int64(bucket.CreateTime.Nanos))
		sst := time.Unix(bucket.StakeStartTime.Seconds, int64(bucket.StakeStartTime.Nanos))
		ust := time.Unix(bucket.UnstakeStartTime.Seconds, int64(bucket.UnstakeStartTime.Nanos))
		if _, err = stmt.Exec(
			hex.EncodeToString(h[:]),
			bucket.Index,
			[]byte(bucket.CandidateAddress),
			[]byte(bucket.Owner),
			[]byte(bucket.StakedAmount),
			duration.String(),
			ct,
			sst,
			ust,
			bucket.AutoStake,
		); err != nil {
			return nil, err
		}
	}

	return frequencies, nil
}

func atos(a []int64) string {
	if len(a) == 0 {
		return ""
	}

	b := make([]string, len(a))
	for i, v := range a {
		b[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(b, ",")
}

func hashBucket(bucket *iotextypes.VoteBucket) (hash.Hash256, error) {
	data, err := proto.Marshal(bucket)
	if err != nil {
		return hash.ZeroHash256, err
	}
	return hash.Hash256b(data), nil
}
