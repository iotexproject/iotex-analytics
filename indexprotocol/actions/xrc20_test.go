// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actions

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-election/pb/api"
	mock_election "github.com/iotexproject/iotex-election/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-analytics/epochctx"
	"github.com/iotexproject/iotex-analytics/indexcontext"
	"github.com/iotexproject/iotex-analytics/indexprotocol"
	"github.com/iotexproject/iotex-analytics/indexprotocol/blocks"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

func TestXrc20(t *testing.T) {
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

	bp := blocks.NewProtocol(store, epochctx.NewEpochCtx(36, 24, 15, epochctx.FairbankHeight(1000000)), indexprotocol.GravityChain{GravityChainStartHeight: 1})
	p := NewProtocol(store, indexprotocol.HermesConfig{
		HermesContractAddress:        "testAddr",
		MultiSendContractAddressList: []string{"testAddr"},
	}, epochctx.NewEpochCtx(36, 24, 15, epochctx.FairbankHeight(1000000)))

	require.NoError(bp.CreateTables(ctx))
	require.NoError(p.CreateTables(ctx))

	chainClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	electionClient := mock_election.NewMockAPIServiceClient(ctrl)
	bpctx := indexcontext.WithIndexCtx(context.Background(), indexcontext.IndexCtx{
		ChainClient:     chainClient,
		ElectionClient:  electionClient,
		ConsensusScheme: "ROLLDPOS",
	})

	electionClient.EXPECT().GetCandidates(gomock.Any(), gomock.Any()).Times(1).Return(
		&api.CandidateResponse{
			Candidates: []*api.Candidate{
				{
					Name:            "616c6661",
					OperatorAddress: testutil.Addr1,
				},
				{
					Name:            "627261766f",
					OperatorAddress: testutil.Addr2,
				},
			},
		}, nil,
	)
	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(indexprotocol.PollProtocolID),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(1, 10))},
	}
	candidateList := state.CandidateList{
		{
			Address:       testutil.Addr1,
			RewardAddress: testutil.RewardAddr1,
			Votes:         big.NewInt(100),
		},
		{
			Address:       testutil.Addr2,
			RewardAddress: testutil.RewardAddr2,
			Votes:         big.NewInt(10),
		},
	}
	data, err := candidateList.Serialize()
	require.NoError(err)
	chainClient.EXPECT().ReadState(gomock.Any(), readStateRequest).Times(1).Return(&iotexapi.ReadStateResponse{
		Data: data,
	}, nil)

	chainClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).AnyTimes().Return(&iotexapi.ReadContractResponse{
		Receipt: &iotextypes.Receipt{Status: 1},
		Data:    "xx",
	}, nil)
	blk, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return bp.HandleBlock(bpctx, tx, blk)
	}))

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(bpctx, tx, blk)
	}))

	// for xrc20
	actionHash := blk.Actions[6].Hash()
	receiptHash := blk.Receipts[6].Hash()
	xrc20History, err := p.getXrc20History("xxxxx")
	require.NoError(err)

	require.Equal(hex.EncodeToString(actionHash[:]), xrc20History[0].ActionHash)
	require.Equal(hex.EncodeToString(receiptHash[:]), xrc20History[0].ReceiptHash)
	require.Equal("xxxxx", xrc20History[0].Address)

	require.Equal(transferSha3, xrc20History[0].Topics)
	require.Equal("0000000000000000000000006356908ace09268130dee2b7de643314bbeb3683000000000000000000000000da7e12ef57c236a06117c5e0d04a228e7181cf360000000000000000000000000000000000000000000000000de0b6b3a7640000", xrc20History[0].Data)
	require.Equal("100000", xrc20History[0].BlockHeight)
	require.Equal("888", xrc20History[0].Index)
	require.Equal("failure", xrc20History[0].Status)

	// for xrc 721
	actionHash = blk.Actions[7].Hash()
	receiptHash = blk.Receipts[7].Hash()
	xrc20History, err = getXrc721History(p, "io1xpvzahnl4h46f9ea6u03ec2hkusrzu020th8xx")
	require.NoError(err)

	require.Equal(hex.EncodeToString(actionHash[:]), xrc20History[0].ActionHash)
	require.Equal(hex.EncodeToString(receiptHash[:]), xrc20History[0].ReceiptHash)
	require.Equal("io1xpvzahnl4h46f9ea6u03ec2hkusrzu020th8xx", xrc20History[0].Address)

	// split 256 `topic` to 192 `topic` & 64 `data`
	require.Equal("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ff003f0d751d3a71172f723fbbc4d262dd47adf0", xrc20History[0].Topics)
	require.Equal("0000000000000000000000000000000000000000000000000000000000000006", xrc20History[0].Data)
	require.Equal("100001", xrc20History[0].BlockHeight)
	require.Equal("666", xrc20History[0].Index)
	require.Equal("failure", xrc20History[0].Status)
}

// getActionHistory returns action history by action hash
func getXrc721History(p *Protocol, address string) ([]*Xrc20History, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf(selectXrc20History, "xrc721_history")
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}
	defer stmt.Close()

	rows, err := stmt.Query(address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute get query")
	}

	var xrc20History Xrc20History
	parsedRows, err := s.ParseSQLRows(rows, &xrc20History)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, indexprotocol.ErrNotExist
	}
	ret := make([]*Xrc20History, 0)
	for _, parsedRow := range parsedRows {
		r := parsedRow.(*Xrc20History)
		ret = append(ret, r)
	}

	return ret, nil
}
