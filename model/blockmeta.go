package model

import (
	iotexsql "github.com/iotexproject/iotex-analytics/sql"
)

type BlockMeta struct {
	iotexsql.Store
}

func NewBlockMeta(store iotexsql.Store) *BlockMeta {
	return &BlockMeta{
		store,
	}
}

func (b *BlockMeta) TableName() string {
	return `block_meta`
}

func (b *BlockMeta) Initialize() error {
	createSql := "CREATE TABLE IF NOT EXISTS `" + b.TableName() + "` (" +
		"`block_height` bigint(20) NOT NULL," +
		"`gas_consumed` int(11) NOT NULL," +
		"`producer_name` varchar(42) NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`block_height`) USING BTREE" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"
	if _, err := b.GetDB().Exec(createSql); err != nil {
		return err
	}
	return nil
}

func (b *BlockMeta) Insert(data Fields) error {
	return InsertTableData(b.GetDB(), b.TableName(), data)
}

func (b *BlockMeta) Update(data Fields, where Fields) error {
	return nil
}
