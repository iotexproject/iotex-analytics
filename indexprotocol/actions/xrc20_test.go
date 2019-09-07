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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

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

	bp := blocks.NewProtocol(store, uint64(24), uint64(36), uint64(15))
	p := NewProtocol(store)

	require.NoError(bp.CreateTables(ctx))
	require.NoError(p.CreateTables(ctx))

	blk, err := testutil.BuildCompleteBlock(uint64(180), uint64(361))
	require.NoError(err)

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return bp.HandleBlock(ctx, tx, blk)
	}))

	require.NoError(store.Transact(func(tx *sql.Tx) error {
		return p.HandleBlock(ctx, tx, blk)
	}))

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
}
