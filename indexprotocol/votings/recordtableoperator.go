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

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-election/committee"
)

// NewVoteBucketTableOperator creates an operator for vote bucket table
func NewBucketTableOperator(tableName string, driverName committee.DRIVERTYPE) (committee.Operator, error) {
	var creation string
	switch driverName {
	case committee.SQLITE:
		creation = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT UNIQUE, index DECIMAL(65, 0), candidate BLOB, owner BLOB, staked_amount BLOB, staked_duration TEXT, create_time TIMESTAMP, stake_start_time TIMESTAMP, unstake_start_time TIMESTAMP, auto_stake INTEGER)"
	case committee.MYSQL:
		creation = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTO_INCREMENT, hash VARCHAR(64) UNIQUE, index DECIMAL(65, 0), candidate BLOB, owner BLOB, staked_amount BLOB, staked_duration TEXT, create_time TIMESTAMP, stake_start_time TIMESTAMP, unstake_start_time TIMESTAMP, auto_stake INTEGER)"
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

// NewCandidateTableOperator create an operator for candidate table
func NewCandidateTableOperator(tableName string, driverName committee.DRIVERTYPE) (committee.Operator, error) {
	var creation string
	switch driverName {
	case committee.SQLITE:
		creation = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT UNIQUE, owner BLOB, operator BLOB, reward BLOB, name BLOB, votes BLOB, self_stake_bucket_idx DECIMAL(65, 0), self_stake BLOB)"
	case committee.MYSQL:
		creation = "CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTO_INCREMENT, hash VARCHAR(64) UNIQUE, owner BLOB, operator BLOB, reward BLOB, name BLOB, votes BLOB, self_stake_bucket_idx DECIMAL(65, 0), self_stake BLOB)"
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

// VoteBucketRecordQuery is query to return vote buckets by ids
const VoteBucketRecordQuery = "SELECT id, index, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake FROM %s WHERE id IN (%s)"

// QueryVoteBuckets returns vote buckets by ids
func QueryVoteBuckets(tableName string, frequencies map[int64]int, sdb *sql.DB, tx *sql.Tx) (interface{}, error) {
	var (
		id, autoStake                                int64
		index                                        uint64
		createTime, stakeStartTime, unstakeStartTime time.Time
		stakedDuration                               string
		owner, candidate, stakedAmount               []byte
		rows                                         *sql.Rows
		err                                          error
	)
	size := 0
	ids := make([]int64, 0, len(frequencies))
	for id, f := range frequencies {
		ids = append(ids, id)
		size += f
	}
	if tx != nil {
		rows, err = tx.Query(fmt.Sprintf(VoteBucketRecordQuery, tableName, atos(ids)))
	} else {
		rows, err = sdb.Query(fmt.Sprintf(VoteBucketRecordQuery, tableName, atos(ids)))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets := make([]*staking.VoteBucket, 0, size)
	for rows.Next() {
		if err := rows.Scan(&id, &index, &candidate, &owner, &stakedAmount, &stakedDuration, &createTime, &stakeStartTime, &unstakeStartTime, &autoStake); err != nil {
			return nil, err
		}
		candAddr, err := address.FromBytes(candidate)
		if err != nil {
			return nil, err
		}
		ownerAddr, err := address.FromBytes(owner)
		if err != nil {
			return nil, err
		}
		duration, err := time.ParseDuration(stakedDuration)
		if err != nil {
			return nil, err
		}
		bucket := &staking.VoteBucket{
			Index:            index,
			Candidate:        candAddr,
			Owner:            ownerAddr,
			StakedAmount:     big.NewInt(0).SetBytes(stakedAmount),
			StakedDuration:   duration,
			CreateTime:       createTime,
			StakeStartTime:   stakeStartTime,
			UnstakeStartTime: unstakeStartTime,
			AutoStake:        autoStake == 1,
		}
		for i := frequencies[id]; i > 0; i-- {
			buckets = append(buckets, bucket)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return buckets, nil
}

// InsertVoteBucketsQuery is query to insert vote buckets
const InsertVoteBucketsQuery = "INSERT OR IGNORE INTO %s (hash, index, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
const InsertVoteBucketsQueryMySql = "INSERT IGNORE INTO %s (hash, index, candidate, owner, staked_amount, staked_duration, create_time, stake_start_time, unstake_start_time, auto_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

// InsertVoteBuckets inserts vote bucket records into table by tx
func InsertVoteBuckets(tableName string, driverName committee.DRIVERTYPE, records interface{}, tx *sql.Tx) (frequencies map[hash.Hash256]int, err error) {
	buckets, ok := records.([]*staking.VoteBucket)
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
	for _, bucket := range buckets {
		h, err := hashBucket(bucket)
		if err != nil {
			return nil, err
		}
		if f, ok := frequencies[h]; ok {
			frequencies[h] = f + 1
		} else {
			frequencies[h] = 1
		}

		if _, err = stmt.Exec(
			hex.EncodeToString(h[:]),
			bucket.Index,
			bucket.Candidate.Bytes(),
			bucket.Owner.Bytes(),
			bucket.StakedAmount.Bytes(),
			bucket.StakedDuration.String(),
			bucket.CreateTime,
			bucket.StakeStartTime,
			bucket.UnstakeStartTime,
			bucket.AutoStake,
		); err != nil {
			return nil, err
		}
	}

	return frequencies, nil
}

// CandidateQuery is query to get candidates by ids
const CandidateQuery = "SELECT id, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake FROM %s WHERE id IN (%s)"

// QueryCandidates get all candidates by ids
func QueryCandidates(tableName string, frequencies map[int64]int, sdb *sql.DB, tx *sql.Tx) (interface{}, error) {
	var (
		rows                                            *sql.Rows
		err                                             error
		owner, operator, reward, name, votes, selfStake []byte
		id                                              int64
		selfStakeBucketIdx                              uint64
	)
	size := 0
	ids := make([]int64, 0, len(frequencies))
	for id, f := range frequencies {
		ids = append(ids, id)
		size += f
	}
	if tx != nil {
		rows, err = tx.Query(fmt.Sprintf(CandidateQuery, tableName, atos(ids)))
	} else {
		rows, err = sdb.Query(fmt.Sprintf(CandidateQuery, tableName, atos(ids)))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]*staking.Candidate, 0, size)
	for rows.Next() {
		if err := rows.Scan(&id, &owner, &operator, &reward, &name, &votes, &selfStakeBucketIdx, &selfStake); err != nil {
			return nil, err
		}
		ownerAddr, err := address.FromBytes(owner)
		if err != nil {
			return nil, err
		}
		operatorAddr, err := address.FromBytes(operator)
		if err != nil {
			return nil, err
		}
		rewardAddr, err := address.FromBytes(reward)
		if err != nil {
			return nil, err
		}
		candidate := &staking.Candidate{
			Owner:              ownerAddr,
			Operator:           operatorAddr,
			Reward:             rewardAddr,
			Name:               string(name),
			Votes:              big.NewInt(0).SetBytes(votes),
			SelfStakeBucketIdx: selfStakeBucketIdx,
			SelfStake:          big.NewInt(0).SetBytes(selfStake),
		}
		for i := frequencies[id]; i > 0; i-- {
			candidates = append(candidates, candidate)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return candidates, nil
}

// InsertCandidateQuerySQLITE is query to insert candidates in SQLITE driver
const InsertCandidateQuerySQLITE = "INSERT OR IGNORE INTO %s (hash, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"

// InsertCandidateQueryMySQL is query to insert candidates in MySQL driver
const InsertCandidateQueryMySQL = "INSERT IGNORE INTO %s (hash, owner, operator, reward, name, votes, self_stake_bucket_idx, self_stake) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"

// InsertCandidates inserts candidate records into table by tx
func InsertCandidates(tableName string, driverName committee.DRIVERTYPE, records interface{}, tx *sql.Tx) (frequencies map[hash.Hash256]int, err error) {
	candidates, ok := records.([]*staking.Candidate)
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
	for _, candidate := range candidates {
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
			candidate.Owner.Bytes(),
			candidate.Operator.Bytes(),
			candidate.Reward.Bytes(),
			[]byte(candidate.Name),
			candidate.Votes.Bytes(),
			candidate.SelfStakeBucketIdx,
			candidate.SelfStake.Bytes(),
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

func hashBucket(bucket *staking.VoteBucket) (hash.Hash256, error) {
	data, err := bucket.Serialize()
	if err != nil {
		return hash.ZeroHash256, err
	}
	return hash.Hash256b(data), nil
}

func hashCandidate(candidate *staking.Candidate) (hash.Hash256, error) {
	data, err := candidate.Serialize()
	if err != nil {
		return hash.ZeroHash256, err
	}
	return hash.Hash256b(data), nil
}
