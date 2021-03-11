package indexer

import (
	"context"
	"database/sql"

	s "github.com/iotexproject/iotex-analytics/sql"
)

/*
CREATE TABLE `index_heights` (
  `name` varchar(128) NOT NULL,
  `height` bigint(20) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
*/
type IndexHeightTable struct {
}

func (iht *IndexHeightTable) Upsert(ctx context.Context, name string, height uint64) error {
	tx, ok := s.ExtractTx(ctx)
	if ok {
		return upsert(tx, name, height)
	}
	store, ok := s.ExtractStore(ctx)
	if !ok {
		return s.ErrNoStoreContext
	}
	return store.Transact(func(tx *sql.Tx) error {
		return upsert(tx, name, height)
	})
}

func upsert(tx *sql.Tx, name string, height uint64) error {
	if _, err := tx.Exec("INSERT IGNORE INTO index_heights (`name`, `height`) VALUES (?, 0)", name); err != nil {
		return err
	}
	_, err := tx.Exec("UPDATE index_heights SET `height` = ? WHERE `name` = ? AND `height` < ?", height, name, height)

	return err
}

func (iht *IndexHeightTable) Height(ctx context.Context, name string) (uint64, error) {
	var row *sql.Row
	tx, ok := s.ExtractTx(ctx)
	if ok {
		row = tx.QueryRow("SELECT height FROM index_heights WHERE name = ?", name)
	} else {
		store, ok := s.ExtractStore(ctx)
		if !ok {
			return 0, s.ErrNoStoreContext
		}
		row = store.GetDB().QueryRow("SELECT height FROM index_heights WHERE name = ?", name)
	}
	var h sql.NullInt64
	if err := row.Scan(&h); err != nil {
		return 0, err
	}
	if !h.Valid {
		return 0, nil
	}
	return uint64(h.Int64), nil
}
