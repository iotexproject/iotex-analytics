package model

import (
	iotexsql "github.com/iotexproject/iotex-analytics/sql"
)

type Block struct {
	iotexsql.Store
}

func NewBlock(store iotexsql.Store) *Block {
	return &Block{
		store,
	}
}

func (b *Block) TableName() string {
	return `block`
}

func (b *Block) Initialize() error {
	createSql := "CREATE TABLE IF NOT EXISTS `" + b.TableName() + "` (" +
		"`block_height` bigint(20) NOT NULL," +
		"`block_hash` varchar(64) NOT NULL," +
		"`producer_address` varchar(42) NOT NULL," +
		"`num_actions` int(6) unsigned NOT NULL DEFAULT '0'," +
		"`timestamp` int(11) unsigned NOT NULL," +
		"PRIMARY KEY (`block_height`) USING BTREE," +
		"KEY `producer_address` (`producer_address`) USING BTREE" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1;"
	if _, err := b.GetDB().Exec(createSql); err != nil {
		return err
	}
	return nil
}

func (b *Block) Insert(f Fields) error {
	return InsertTableData(b.GetDB(), b.TableName(), f)
}

func (b *Block) Update(data Fields, where Fields) error {
	return nil
}
