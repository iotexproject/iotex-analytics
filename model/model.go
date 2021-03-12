package model

import (
	"database/sql"
	"fmt"
	"strings"

	iotexsql "github.com/iotexproject/iotex-analytics/sql"
)

type (
	Fields map[string]interface{}

	Schema interface {
		iotexsql.Store
		Initialize() error
		Insert(data Fields) error
		Update(data Fields, where Fields) error
		TableName() string
	}
)

func InsertTableData(db *sql.DB, table string, data map[string]interface{}) error {
	var cols, vals []string
	var x []interface{}
	for k, v := range data {
		cols = append(cols, fmt.Sprintf("%s", k))
		vals = append(vals, "?")
		x = append(x, v)
	}
	sqlStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(cols, ","), strings.Join(vals, ","))
	stmt, err := db.Prepare(sqlStr)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(x...); err != nil {
		return err
	}
	return nil

}
