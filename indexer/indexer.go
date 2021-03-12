package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-core/blockchain/block"
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

func insertTableData(tx *sql.Tx, table string, data map[string]interface{}) error {
	var cols, vals []string
	var x []interface{}
	for k, v := range data {
		cols = append(cols, fmt.Sprintf("%s", k))
		vals = append(vals, "?")
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
