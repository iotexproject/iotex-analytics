// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewards

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName     = "heroku_7fed0b046078f80"
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

	p := NewProtocol(store, epochctx.NewEpochCtx(1, 1, 1, epochctx.FairbankHeight(100000)), indexprotocol.Rewarding{})

	require.NoError(p.CreateTables(ctx))

	blk1, err := testutil.BuildCompleteBlock(uint64(1), uint64(2))
	require.NoError(err)

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	ctx = indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: "ROLLDPOS",
	})

	chainClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: []byte(strconv.FormatUint(1000, 10)),
	}, nil)
	electionClient.EXPECT().GetCandidates(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.CandidateResponse{
			Candidates: []*api.Candidate{
				{
					Name:          "616c6661",
					RewardAddress: testutil.RewardAddr1,
				},
				{
					Name:          "627261766f",
					RewardAddress: testutil.RewardAddr2,
				},
				{
					Name:          "636861726c6965",
					RewardAddress: testutil.RewardAddr3,
				},
			},
		}, nil,
	)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk1)
	}))

	actionHash1 := blk1.Actions[4].Hash()
	rewardHistoryList, err := p.getRewardHistory(hex.EncodeToString(actionHash1[:]))
	require.NoError(err)
	require.Equal(1, len(rewardHistoryList))
	require.Equal(uint64(1), rewardHistoryList[0].EpochNumber)
	require.Equal("616c6661", rewardHistoryList[0].CandidateName)
	require.Equal(testutil.RewardAddr1, rewardHistoryList[0].RewardAddress)
	require.Equal("16", rewardHistoryList[0].BlockReward)
	require.Equal("0", rewardHistoryList[0].EpochReward)
	require.Equal("0", rewardHistoryList[0].FoundationBonus)

	actionHash2 := blk1.Actions[5].Hash()
	rewardHistoryList, err = p.getRewardHistory(hex.EncodeToString(actionHash2[:]))
	require.NoError(err)
	require.Equal(3, len(rewardHistoryList))
}
