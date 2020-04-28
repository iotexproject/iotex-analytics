// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package votings

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	//localconnectStr = "root:123456@tcp(192.168.146.140:3306)/"
	localconnectStr = connectStr
	//localdbName     = "analytics"
	localdbName = dbName
)

var (
	now     = time.Now()
	buckets = []*iotextypes.VoteBucket{
		&iotextypes.VoteBucket{
			Index:            10,
			CandidateAddress: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			StakedAmount:     "30000",
			StakedDuration:   86400, // one day
			CreateTime:       &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			StakeStartTime:   &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			UnstakeStartTime: &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			AutoStake:        false,
			Owner:            "io1l9vaqmanwj47tlrpv6etf3pwq0s0snsq4vxke2",
		},
		&iotextypes.VoteBucket{
			Index:            11,
			CandidateAddress: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			StakedAmount:     "30000",
			StakedDuration:   86400,
			CreateTime:       &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			StakeStartTime:   &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			UnstakeStartTime: &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			AutoStake:        false,
			Owner:            "io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02",
		},
		&iotextypes.VoteBucket{
			Index:            12,
			CandidateAddress: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			StakedAmount:     "30000",
			StakedDuration:   86400,
			CreateTime:       &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			StakeStartTime:   &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			UnstakeStartTime: &timestamp.Timestamp{Seconds: now.Unix(), Nanos: 0},
			AutoStake:        false,
			Owner:            "io1vdtfpzkwpyngzvx7u2mauepnzja7kd5rryp0sg",
		},
	}
	candidates = []*iotextypes.CandidateV2{
		&iotextypes.CandidateV2{
			OwnerAddress:       "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			OperatorAddress:    "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			RewardAddress:      "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			Name:               delegateName,
			TotalWeightedVotes: "10",
			SelfStakeBucketIdx: 6666,
			SelfStakingTokens:  "99999",
		},
	}
	delegateName = "xxxx"
)

func TestStakingV2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	mock(chainClient, t)
	height := uint64(110000)
	epochNumber := uint64(68888)
	require := require.New(t)
	ctx := context.Background()
	//use for remote database
	testutil.CleanupDatabase(t, connectStr, dbName)
	store := s.NewMySQL(localconnectStr, localdbName)
	require.NoError(store.Start(ctx))
	defer func() {
		//use for remote database
		_, err := store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()
	require.NoError(store.Start(context.Background()))
	cfg := indexprotocol.VoteWeightCalConsts{
		DurationLg: 1.2,
		AutoStake:  1,
		SelfStake:  1.05,
	}
	p, err := NewProtocol(store, epochctx.NewEpochCtx(36, 24, 15), indexprotocol.GravityChain{}, indexprotocol.Poll{
		VoteThreshold:        "100000000000000000000",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	}, cfg)
	require.NoError(err)
	require.NoError(p.CreateTables(context.Background()))
	require.NoError(p.stakingV2(chainClient, height, epochNumber, nil))

	// case I: checkout bucket if it's written right
	require.NoError(err)
	ret, err := p.nativeV2BucketTableOperator.Get(height, p.Store.GetDB(), nil)
	require.NoError(err)
	bucketList, ok := ret.(*iotextypes.VoteBucketList)
	require.True(ok)
	bucketsBytes, _ := proto.Marshal(&iotextypes.VoteBucketList{Buckets: buckets})
	bucketListBytes, _ := proto.Marshal(bucketList)
	require.EqualValues(bucketsBytes, bucketListBytes)

	// case II: checkout candidate if it's written right
	ret, err = p.nativeV2CandidateTableOperator.Get(height, p.Store.GetDB(), nil)
	require.NoError(err)
	candidateList, ok := ret.(*iotextypes.CandidateListV2)
	require.True(ok)

	candidatesBytes, _ := proto.Marshal(&iotextypes.CandidateListV2{Candidates: candidates})
	candidateListBytes, _ := proto.Marshal(candidateList)
	require.EqualValues(candidatesBytes, candidateListBytes)

	// case III: check getBucketInfoByEpochV2
	bucketInfo, err := p.getBucketInfoByEpochV2(height, epochNumber, delegateName)
	require.NoError(err)
	require.Equal("io1l9vaqmanwj47tlrpv6etf3pwq0s0snsq4vxke2", bucketInfo[0].VoterAddress)
	require.Equal("io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02", bucketInfo[1].VoterAddress)
	require.Equal("io1vdtfpzkwpyngzvx7u2mauepnzja7kd5rryp0sg", bucketInfo[2].VoterAddress)
	for _, b := range bucketInfo {
		require.True(b.Decay)
		require.Equal(epochNumber, b.EpochNumber)
		require.True(b.IsNative)
		dur, err := strconv.ParseFloat(b.RemainingDuration, 64)
		require.NoError(err)
		require.True(uint64(dur) <= uint64(86400))
		require.Equal(fmt.Sprintf("%d", now.Unix()), b.StartTime)
		require.Equal("30000", b.Votes)
		require.Equal("30000", b.WeightedVotes)
	}
}

func TestRemainingTime(t *testing.T) {
	require := require.New(t)
	// case I: now is before start time
	bucketTime := time.Now().Add(time.Second * 100)
	timestamp, _ := ptypes.TimestampProto(bucketTime)
	bucket := &iotextypes.VoteBucket{
		StakeStartTime: timestamp,
		StakedDuration: 100,
	}
	remaining := remainingTime(bucket)
	require.Equal(time.Duration(0), remaining)

	// case II: now is between start time and starttime+stakedduration
	bucketTime = time.Unix(time.Now().Unix()-10, 0)
	timestamp, _ = ptypes.TimestampProto(bucketTime)
	bucket = &iotextypes.VoteBucket{
		StakeStartTime: timestamp,
		StakedDuration: 100,
	}
	remaining = remainingTime(bucket)
	require.True(remaining > 0)

	// case III: now is after starttime+stakedduration
	bucketTime = time.Unix(time.Now().Unix()-200, 0)
	timestamp, _ = ptypes.TimestampProto(bucketTime)
	bucket = &iotextypes.VoteBucket{
		StakeStartTime: timestamp,
		StakedDuration: 100,
	}
	remaining = remainingTime(bucket)
	require.Equal(time.Duration(0), remaining)
}

func TestFilterCandidatesV2(t *testing.T) {
	require := require.New(t)
	cl := &iotextypes.CandidateListV2{Candidates: candidates}
	unqualifiedList := &iotextypes.ProbationCandidateList{
		IntensityRate: 10,
		ProbationList: []*iotextypes.ProbationCandidateList_Info{
			&iotextypes.ProbationCandidateList_Info{
				Address: "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				Count:   10,
			},
		},
	}
	ret, err := filterCandidates(cl, unqualifiedList, 10)
	require.NoError(err)
	cl, ok := ret.(*iotextypes.CandidateListV2)
	require.True(ok)
	require.Equal("9", cl.Candidates[0].TotalWeightedVotes)
}

func mock(chainClient *mock_apiserviceclient.MockServiceClient, t *testing.T) {
	require := require.New(t)
	methodNameBytes, _ := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS,
	})
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: 0,
					Limit:  readBucketsLimit,
				},
			},
		},
	})
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(protocolID),
		MethodName: methodNameBytes,
		Arguments:  [][]byte{arg},
	}

	vbl := &iotextypes.VoteBucketList{Buckets: buckets}
	s, err := proto.Marshal(vbl)
	require.NoError(err)
	first := chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).AnyTimes().Return(&iotexapi.ReadStateResponse{
		Data: s,
	}, nil)

	// mock candidate
	methodNameBytes, _ = proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	})
	arg, err = proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
				Pagination: &iotexapi.PaginationParam{
					Offset: 0,
					Limit:  readCandidatesLimit,
				},
			},
		},
	})
	readStateRequest = &iotexapi.ReadStateRequest{
		ProtocolID: []byte(protocolID),
		MethodName: methodNameBytes,
		Arguments:  [][]byte{arg},
	}

	cl := &iotextypes.CandidateListV2{Candidates: candidates}
	s, err = proto.Marshal(cl)
	require.NoError(err)
	second := chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).AnyTimes().Return(&iotexapi.ReadStateResponse{
		Data: s,
	}, nil)

	gomock.InOrder(
		first,
		second,
	)
}
