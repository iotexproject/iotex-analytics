package actions

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-analytics/indexservice"
	s "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-analytics/testutil"
)

const (
	connectStr             = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName                 = "heroku_7fed0b046078f80"
	ActionHistoryTableName = "action_history"
	Xrc20HistoryTableName  = "xrc20_history"
)

func TestProtocol(t *testing.T) {

	// Creating temporary store
	require := require.New(t)
	ctx := context.Background()
	var err error
	var errA error
	var errXrc error

	testutil.CleanupDatabase(t, connectStr, dbName)

	store := s.NewMySQL(connectStr, dbName)
	require.NoError(store.Start(ctx))
	defer func() {
		_, err = store.GetDB().Exec("DROP DATABASE " + dbName)
		require.NoError(err)
		require.NoError(store.Stop(ctx))
	}()

	// Creating protocol
	var cfg indexservice.Config
	idx := indexservice.NewIndexer(store, cfg)
	p := NewProtocol(idx)

	// Testing unregistered
	t.Run("Testing unregistered", func(t *testing.T) {
		_, errA = p.GetActiveAccount(1)
		require.EqualError(errA, "actions protocol is unregistered")

		_, errXrc = p.GetXrc20("", 1, 1)
		require.Error(errXrc)
		require.EqualError(errXrc, "actions protocol is unregistered")
	})

	idx.RegisterDefaultProtocols()

	// Testing no tables
	t.Run("Testing no table", func(t *testing.T) {
		_, errA = p.GetActiveAccount(1)
		require.EqualError(errA, "failed to prepare get query: "+
			"Error 1146: Table 'heroku_7fed0b046078f80.action_history' doesn't exist")

		_, errXrc = p.GetXrc20("", 1, 1)
		require.Error(errXrc)
		require.EqualError(errXrc, "failed to prepare get query: Error 1146: Table 'heroku_7fed0b046078f80.xrc20_history' doesn't exist")
	})

	// Testing empty tables
	t.Run("Testing empty table", func(t *testing.T) {
		// Creating tables
		_, errA = store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
			"(block_height DECIMAL(65, 0), "+
			"`from` VARCHAR(41))",
			ActionHistoryTableName))
		require.NoError(errA)

		_, errXrc := store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
			"(action_hash VARCHAR(64) NOT NULL, receipt_hash VARCHAR(64) NOT NULL UNIQUE, address VARCHAR(41) NOT NULL,"+
			"`topics` VARCHAR(192),`data` VARCHAR(192),block_height DECIMAL(65, 0), `index` DECIMAL(65, 0),"+
			"`timestamp` DECIMAL(65, 0),status VARCHAR(7) NOT NULL, PRIMARY KEY (action_hash,receipt_hash,topics))",
			Xrc20HistoryTableName))
		require.NoError(errXrc)

		_, errA = p.GetActiveAccount(1)
		require.EqualError(errA, "not exist in DB")

		_, errXrc = p.GetXrc20("", 1, 1)
		require.Error(errXrc)
		require.EqualError(errXrc, "not exist in DB")
	})

	// Populate ActionHistory
	type testDataA struct {
		input    int
		listSize int
		hList    []uint64
		fList    []string
		output   []string
	}

	var testSituationsA = []testDataA{
		{2, 3, []uint64{1, 2, 3}, []string{"a", "b", "c"}, []string{"c", "b"}},
		{2, 3, []uint64{1, 2, 2}, []string{"a", "b", "b"}, []string{"b", "a"}},
		{2, 3, []uint64{3, 2, 1}, []string{"a", "b", "c"}, []string{"a", "b"}},
	}

	for _, tCase := range testSituationsA {
		valStrs := make([]string, 0, tCase.listSize)
		valArgs := make([]interface{}, 0, tCase.listSize*2)
		for i := 0; i < tCase.listSize; i++ {
			valStrs = append(valStrs, "(?, ?)")
			valArgs = append(valArgs, tCase.hList[i], tCase.fList[i])
		}

		_, errA = store.GetDB().Exec("SET AUTOCOMMIT = 1")
		insertQuery := fmt.Sprintf("INSERT INTO %s (block_height, `from`) VALUES %s", ActionHistoryTableName, strings.Join(valStrs, ","))
		_, errA = store.GetDB().Exec(insertQuery, valArgs...)
		require.NoError(errA)

		t.Run("Testing Active Account data cases", func(t *testing.T) {
			test, errA := p.GetActiveAccount(tCase.input)
			require.NoError(errA)
			require.ElementsMatch(test, tCase.output)
		})

		// Drop table
		_, errA = store.GetDB().Exec("DROP TABLE " + ActionHistoryTableName)
		require.NoError(errA)

		// Create empty table
		_, errA = store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
			"(block_height DECIMAL(65, 0), "+
			"`from` VARCHAR(41))",
			ActionHistoryTableName))
		require.NoError(errA)
	}

	// Populate Xrc20History
	type testDataXrc struct {
		inputA          string
		inputNPP        uint64
		inputP          uint64
		listSize        int
		actionHashList  []string
		receiptHashList []string
		addressList     []string
		topicsList      []string
		dataList        []string
		blockHeightList []uint64
		indexList       []uint64
		timestampList   []uint64
		statusList      []string
		output          []*Xrc20Info
	}

	contract := []*Xrc20Info{
		{
			"0037c290ee2a21faa0ea5ebd7975b6b85088a7409715267e359625e60e399da5",
			"1565825770",
			"io1lkg0gxx4a79khszeawsshtng5tcax6v9dx8hna",
			"io1rxwu5wd24y2nf5t836uu7yky85p5f0pvquj59m",
			"10000000000000000000",
		},
		{
			"003c40bb33f7bd682ae77d8fdd7f63d719a468f6eabd3163773602f5b5ce683b",
			"1565729760",
			"io1lkg0gxx4a79khszeawsshtng5tcax6v9dx8hna",
			"io1rxwu5wd24y2nf5t836uu7yky85p5f0pvquj59m",
			"10000000000000000000",
		},
		{
			"0029638f5f82b1d23a4bf8f059e2a2bffe5c07733de681afea41b1cf931b1580",
			"1565391390",
			"io1lkg0gxx4a79khszeawsshtng5tcax6v9dx8hna",
			"io1rxwu5wd24y2nf5t836uu7yky85p5f0pvquj59m",
			"10000000000000000000",
		},
	}

	var testSituation = testDataXrc{
		"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw",
		3,
		1,
		3,
		[]string{"0037c290ee2a21faa0ea5ebd7975b6b85088a7409715267e359625e60e399da5", "003c40bb33f7bd682ae77d8fdd7f63d719a468f6eabd3163773602f5b5ce683b", "0029638f5f82b1d23a4bf8f059e2a2bffe5c07733de681afea41b1cf931b1580"},
		[]string{"4840ac38e3bb1dbcbcb69132947a536a6cc3730b329b20b72cca02e8ec7ddab0", "fd7ce9ba41b0caf8dc79e10d09a7b06fe4c28ff59ef62c224ba5bd0bd894ade5", "ae268c123a5123f7c7bcbadbea1356fc8ccea62a632598cdcc1c1d29c55a6591"},
		[]string{"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw", "io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw", "io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw"},
		[]string{"ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000fd90f418d5ef8b6bc059eba10bae68a2f1d36985000000000000000000000000199dca39aaa91534d1678eb9cf12c43d0344bc2c", "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000fd90f418d5ef8b6bc059eba10bae68a2f1d36985000000000000000000000000199dca39aaa91534d1678eb9cf12c43d0344bc2c", "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000fd90f418d5ef8b6bc059eba10bae68a2f1d36985000000000000000000000000199dca39aaa91534d1678eb9cf12c43d0344bc2c"},
		[]string{"0000000000000000000000000000000000000000000000008ac7230489e80000", "0000000000000000000000000000000000000000000000008ac7230489e80000", "0000000000000000000000000000000000000000000000008ac7230489e80000"},
		[]uint64{985114, 975627, 941871},
		[]uint64{0, 0, 0},
		[]uint64{1565825770, 1565729760, 1565391390},
		[]string{"success", "success", "success"},
		contract,
	}

	valStrs := make([]string, 0, testSituation.listSize)
	valArgs := make([]interface{}, 0, testSituation.listSize*2)
	for i := 0; i < testSituation.listSize; i++ {
		valStrs = append(valStrs, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		valArgs = append(valArgs, testSituation.actionHashList[i], testSituation.receiptHashList[i], testSituation.addressList[i], testSituation.topicsList[i], testSituation.dataList[i], testSituation.blockHeightList[i], testSituation.indexList[i], testSituation.timestampList[i], testSituation.statusList[i])
	}

	_, errXrc = store.GetDB().Exec("SET AUTOCOMMIT = 1")
	insertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (action_hash, receipt_hash, address,topics,`data`,block_height, `index`,`timestamp`,status) VALUES %s", Xrc20HistoryTableName, strings.Join(valStrs, ","))
	_, errXrc = store.GetDB().Exec(insertQuery, valArgs...)
	require.NoError(errXrc)

	t.Run("Testing data cases", func(t *testing.T) {
		test, errXrc := p.GetXrc20(testSituation.inputA, testSituation.inputNPP, testSituation.inputP)
		require.NoError(errXrc)
		for k := 0; k < testSituation.listSize; k++ {
			require.Equal(test[k].Hash, testSituation.output[k].Hash)
			require.Equal(test[k].From, testSituation.output[k].From)
			require.Equal(test[k].To, testSituation.output[k].To)
			require.Equal(test[k].Quantity, testSituation.output[k].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k].Timestamp)
		}
	})

}
