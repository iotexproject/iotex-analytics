// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"database/sql"
	"encoding/hex"
	//"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/db"
	"github.com/iotexproject/iotex-election/pb/api"
	"github.com/iotexproject/iotex-election/pb/election"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName     = "heroku_7fed0b046078f80"
	selectAggregateVoting    = "SELECT aggregate_votes FROM %s WHERE epoch_number=? AND candidate_name=? AND voter_address=?"
	selectVotingMeta 		 = "SELECT total_weighted FROM %s WHERE epoch_number=?"
)

func TestProtocol(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupDatabase(t, connectStr, dbName)
	store := s.NewMySQL(connectStr, dbName)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()
	cfg := indexprotocol.VoteWeightCalConsts{}
	p, err := NewProtocol(store, epochctx.NewEpochCtx(36, 24, 15, epochctx.FairbankHeight(100000)), indexprotocol.GravityChain{}, indexprotocol.Poll{
		VoteThreshold:        "0",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	}, cfg)
	require.NoError(err)
	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(361), uint64(721))
	require.NoError(err)

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	ctx = indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: "ROLLDPOS",
	})
	// first call GetGravityChainStartHeight
	readStateRequestForGravityHeight := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("GetGravityChainStartHeight"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(1, 10))},
	}
	first := chainClient.EXPECT().ReadState(gomock.Any(), readStateRequestForGravityHeight).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: []byte(strconv.FormatUint(1000, 10)),
	}, nil)
	// second call ProbationListByEpoch
	probationListByEpochRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("ProbationListByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(2, 10))},
	}
	pb := &iotextypes.ProbationCandidateList{
		IntensityRate: uint32(90),
		ProbationList: []*iotextypes.ProbationCandidateList_Info {
			{
				Address: testutil.Addr1,	
				Count:	 uint32(1),
			},	
		},
	}
	data, err := proto.Marshal(pb)
	second := chainClient.EXPECT().ReadState(gomock.Any(), probationListByEpochRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: data,
	}, nil)
	gomock.InOrder(
		second,
		first,
	)
	timestamp, err := ptypes.TimestampProto(time.Unix(1000, 0))
	require.NoError(err)

	chainClient.EXPECT().GetElectionBuckets(gomock.Any(), gomock.Any()).Times(1).Return(&iotexapi.GetElectionBucketsResponse{
		Buckets: []*iotextypes.ElectionBucket{},
	}, db.ErrNotExist)
	name1, err := hex.DecodeString("abcd")
	require.NoError(err)
	name2, err := hex.DecodeString("1234")
	require.NoError(err)

	voter1, err := hex.DecodeString("11")
	require.NoError(err)
	voter2, err := hex.DecodeString("22")
	require.NoError(err)
	voter3, err := hex.DecodeString("33")
	require.NoError(err)

	electionClient.EXPECT().GetRawData(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.RawDataResponse{
			Timestamp: timestamp,
			Buckets: []*election.Bucket{
				{
					Voter:     voter1,
					Candidate: name1,
					StartTime: timestamp,
					Duration:  ptypes.DurationProto(time.Duration(10 * 24)),
					Decay:     true,
					Amount:    new(big.Int).SetInt64(100).Bytes(),
				},
				{
					Voter:     voter2,
					Candidate: name1,
					StartTime: timestamp,
					Duration:  ptypes.DurationProto(time.Duration(10 * 24)),
					Decay:     true,
					Amount:    new(big.Int).SetInt64(50).Bytes(),
				},
				{
					Voter:     voter3,
					Candidate: name2,
					StartTime: timestamp,
					Duration:  ptypes.DurationProto(time.Duration(10 * 24)),
					Decay:     true,
					Amount:    new(big.Int).SetInt64(100).Bytes(),
				},
			},
			Registrations: []*election.Registration{
				{
					Name:              name1,
					Address:           []byte("112233"),
					OperatorAddress:   []byte(testutil.Addr1),
					RewardAddress:     []byte(testutil.RewardAddr1),
					SelfStakingWeight: 100,
				},
				{
					Name:              name2,
					Address:           []byte("445566"),
					OperatorAddress:   []byte(testutil.Addr2),
					RewardAddress:     []byte(testutil.RewardAddr2),
					SelfStakingWeight: 102,
				},
			},
		}, nil,
	)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))
	// Probation Test 
	// VotingResult  
	res1, err := p.GetVotingResult(2, "abcd")
	require.NoError(err)
	res2, err := p.GetVotingResult(2, "1234")	
	require.NoError(err)
	require.Equal("abcd", res1.DelegateName)
	require.Equal("1234", res2.DelegateName)
	require.Equal("15", res1.TotalWeightedVotes) // (100 + 50) * 0.1
	require.Equal("100", res2.TotalWeightedVotes)

	/*
	// takes too long time to pass it, need further investigate
	// AggregateVoting  
	getQuery := fmt.Sprintf(selectAggregateVoting, AggregateVotingTableName)
	stmt, err := store.GetDB().Prepare(getQuery)
	require.NoError(err)
	defer stmt.Close()
	var weightedVotes uint64 
	require.NoError(stmt.QueryRow(2, "abcd", "11").Scan(&weightedVotes)) 
	require.Equal(uint64(10), weightedVotes) // 100 * 0.1
	// VotingMeta 
	getQuery = fmt.Sprintf(selectVotingMeta, VotingMetaTableName)
	stmt, err = store.GetDB().Prepare(getQuery)
	require.NoError(err)
	defer stmt.Close()
	var totalWeightedVotes string
	require.NoError(stmt.QueryRow(2).Scan(&totalWeightedVotes))
	require.Equal("115", totalWeightedVotes)
	*/
}
