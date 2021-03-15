package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	iotexsql "github.com/iotexproject/iotex-analytics/sql"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/pkg/errors"
)

type (
	H map[string]interface{}

	// AsyncIndexer is the interface of an async indexer of analytics
	AsyncIndexer interface {
		Start(context.Context) error
		Stop(context.Context) error
		NextHeight(context.Context) (uint64, error)
		PutBlock(context.Context, *block.Block) error
	}
)

func insertTableData(tx Transaction, table string, data map[string]interface{}) error {
	var cols, vals []string
	var x []interface{}
	for k, v := range data {
		cols, vals = append(cols, k), append(vals, "?")
		x = append(x, v)
	}
	sqlStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(cols, ","), strings.Join(vals, ","))
	stmt, err := tx.Prepare(sqlStr)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(x...); err != nil {
		return err
	}
	return nil

}

func getDB(ctx context.Context) *sql.DB {
	store, ok := iotexsql.ExtractStore(ctx)
	if !ok {
		panic(iotexsql.ErrNoStoreContext)
	}
	return store.GetDB()
}
func getIndexHeight(db *sql.DB, name string) (uint64, error) {
	row := db.QueryRow("SELECT height FROM index_heights WHERE name = ?", name)

	var h sql.NullInt64
	if err := row.Scan(&h); err != nil && err != sql.ErrNoRows {
		return 0, errors.Wrapf(err, "failed to get index height :%s", name)
	}
	if !h.Valid {
		return 0, nil
	}
	return uint64(h.Int64), nil
}

func updateIndexHeight(tx Transaction, name string, height uint64) error {
	if _, err := tx.Exec("INSERT INTO index_heights (`name`, `height`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `height` = ?", name, height, height); err != nil {
		return err
	}
	return nil
}
