package actions

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-analytics/indexprotocol"
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
	cfg.Poll = indexprotocol.Poll{
		VoteThreshold:        "100000000000000000000",
		ScoreThreshold:       "0",
		SelfStakingThreshold: "0",
	}
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
			"io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7",
			"io12maua56ssks7xxqkzqumhrg9m6r3rlmu2mcgr7",
			"768472698099944258074",
			"1565825770",
			"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw",
		},
		{
			"003c40bb33f7bd682ae77d8fdd7f63d719a468f6eabd3163773602f5b5ce683b",
			"io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7",
			"io1q77yjjytau2t0yrs656djzu6xe63umuj9afq6j",
			"63615661248503025484915",
			"1565729760",
			"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw",
		},
		{
			"0029638f5f82b1d23a4bf8f059e2a2bffe5c07733de681afea41b1cf931b1580",
			"io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7",
			"io13k7t6k3excy3hkyp60zjakc5le5fvvapqcwfsg",
			"227184383802042044417",
			"1565391390",
			"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw",
		},
		{
			"e740ce8ee6f5419497461453d4613fbaa676f493791026dc60417f489222959a",
			"io1rxwu5wd24y2nf5t836uu7yky85p5f0pda2xpcf",
			"io13k7t6k3excy3hkyp60zjakc5le5fvvapqcwfsg",
			"227184383802042044417",
			"1565291380",
			"io14j96vg9pkx28htpgt2jx0tf3v9etpg4j9h384m",
		},
	}

	var testSituation = testDataXrc{
		"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw",
		3,
		1,
		4,
		[]string{"0029638f5f82b1d23a4bf8f059e2a2bffe5c07733de681afea41b1cf931b1580", "0037c290ee2a21faa0ea5ebd7975b6b85088a7409715267e359625e60e399da5", "003c40bb33f7bd682ae77d8fdd7f63d719a468f6eabd3163773602f5b5ce683b",
			"e740ce8ee6f5419497461453d4613fbaa676f493791026dc60417f489222959a"},
		[]string{"4840ac38e3bb1dbcbcb69132947a536a6cc3730b329b20b72cca02e8ec7ddab0", "fd7ce9ba41b0caf8dc79e10d09a7b06fe4c28ff59ef62c224ba5bd0bd894ade5", "ae268c123a5123f7c7bcbadbea1356fc8ccea62a632598cdcc1c1d29c55a6591", "e740ce8ee6f5419497461453d4613fbaa676f493791026dc60417f4892229590"},
		[]string{"io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw", "io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw", "io1hp6y4eqr90j7tmul4w2wa8pm7wx462hq0mg4tw", "io14j96vg9pkx28htpgt2jx0tf3v9etpg4j9h384m"},
		[]string{"ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef00000000000000000000000000000000000000000000000000000000000000000000000000000000000000008dbcbd5a3936091bd881d3c52edb14fe689633a1", "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000000000000000000000000000000000000000000000000000000000000000000056fbced35085a1e318161039bb8d05de8711ff7c", "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007bc49488bef14b79070d534d90b9a36751e6f92", "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000199dca39aaa91534d1678eb9cf12c43d0344bc2d0000000000000000000000008dbcbd5a3936091bd881d3c52edb14fe689633a1"},
		[]string{"00000000000000000000000000000000000000000000000c50d11164bcb95001", "000000000000000000000000000000000000000000000029a8b37761092dfa1a", "000000000000000000000000000000000000000000000d789ca5e74b7db3c473", "00000000000000000000000000000000000000000000000c50d11164bcb95001"},
		[]uint64{941871, 985114, 975627, 930000},
		[]uint64{0, 0, 0, 0},
		[]uint64{1565391390, 1565825770, 1565729760, 1565291380},
		[]string{"success", "success", "success", "success"},
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

	t.Run("Testing GetXrc20 by contract address", func(t *testing.T) {
		test, errXrc := p.GetXrc20(testSituation.inputA, testSituation.inputNPP, testSituation.inputP)
		require.NoError(errXrc)
		for k := 0; k < 3; k++ {
			require.Equal(test[k].Hash, testSituation.output[k].Hash)
			require.Equal(test[k].From, testSituation.output[k].From)
			require.Equal(test[k].To, testSituation.output[k].To)
			require.Equal(test[k].Quantity, testSituation.output[k].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k].Contract)
		}
	})

	// test case for get xrc20 info by sender or recipient address
	t.Run("Testing GetXrc20 by sender or recipient address", func(t *testing.T) {
		test, errXrc := p.GetXrc20ByAddress("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7", 4, 1)
		require.NoError(errXrc)
		for k := 0; k < 3; k++ {
			require.Equal(test[k].Hash, testSituation.output[k].Hash)
			require.Equal(test[k].From, testSituation.output[k].From)
			require.Equal(test[k].To, testSituation.output[k].To)
			require.Equal(test[k].Quantity, testSituation.output[k].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k].Contract)
		}
	})
	t.Run("Testing GetXrc20 by sender or recipient address", func(t *testing.T) {
		test, errXrc := p.GetXrc20ByAddress("io13k7t6k3excy3hkyp60zjakc5le5fvvapqcwfsg", 4, 1)
		require.NoError(errXrc)
		for k := 0; k < 2; k++ {
			require.Equal(test[k].Hash, testSituation.output[k+2].Hash)
			require.Equal(test[k].From, testSituation.output[k+2].From)
			require.Equal(test[k].To, testSituation.output[k+2].To)
			require.Equal(test[k].Quantity, testSituation.output[k+2].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k+2].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k+2].Contract)
		}
	})
	// test case for get xrc20 info by page
	t.Run("Testing GetXrc20 by page", func(t *testing.T) {
		test, errXrc := p.GetXrc20ByPage(4, 1)
		require.NoError(errXrc)
		for k := 0; k < 4; k++ {
			require.Equal(test[k].Hash, testSituation.output[k].Hash)
			require.Equal(test[k].From, testSituation.output[k].From)
			require.Equal(test[k].To, testSituation.output[k].To)
			require.Equal(test[k].Quantity, testSituation.output[k].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k].Contract)
		}
	})
	t.Run("Testing GetXrc20 by page", func(t *testing.T) {
		test, errXrc := p.GetXrc20ByPage(2, 1)
		require.NoError(errXrc)
		for k := 0; k < 2; k++ {
			require.Equal(test[k].Hash, testSituation.output[k].Hash)
			require.Equal(test[k].From, testSituation.output[k].From)
			require.Equal(test[k].To, testSituation.output[k].To)
			require.Equal(test[k].Quantity, testSituation.output[k].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k].Contract)
		}
	})
	t.Run("Testing GetXrc20 by page", func(t *testing.T) {
		test, errXrc := p.GetXrc20ByPage(2, 2)
		require.NoError(errXrc)
		for k := 0; k < 2; k++ {
			require.Equal(test[k].Hash, testSituation.output[k+2].Hash)
			require.Equal(test[k].From, testSituation.output[k+2].From)
			require.Equal(test[k].To, testSituation.output[k+2].To)
			require.Equal(test[k].Quantity, testSituation.output[k+2].Quantity)
			require.Equal(test[k].Timestamp, testSituation.output[k+2].Timestamp)
			require.Equal(test[k].Contract, testSituation.output[k+2].Contract)
		}
	})
}
