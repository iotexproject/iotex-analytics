// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	// this is required for mysql usage
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	// ErrNoStoreContext no store in the context
	ErrNoStoreContext = errors.New("no store in context")
)

func WithStore(ctx context.Context, store Store) context.Context {
	return context.WithValue(ctx, storeKey{}, store)
}

func ExtractStore(ctx context.Context) (Store, bool) {
	store, ok := ctx.Value(storeKey{}).(Store)
	return store, ok
}

func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func ExtractTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

type (
	txKey    struct{}
	storeKey struct{}

	// Store is the interface of KV store.
	Store interface {
		lifecycle.StartStopper

		// Get DB instance
		GetDB() *sql.DB

		// Transact wrap the transaction
		Transact(txFunc func(*sql.Tx) error) (err error)
	}

	// storebase is a MySQL instance
	storeBase struct {
		mutex      sync.RWMutex
		db         *sql.DB
		connectStr string
		dbName     string
		driverName string
	}
)

// logger is initialized with default settings
var logger = zerolog.New(os.Stderr).Level(zerolog.InfoLevel).With().Timestamp().Logger()

// NewStoreBase instantiates an store base
func newStoreBase(driverName string, connectStr string, dbName string) Store {
	return &storeBase{db: nil, connectStr: connectStr, dbName: dbName, driverName: driverName}
}

// Start opens the SQL (creates new file if not existing yet)
func (s *storeBase) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db != nil {
		return nil
	}

	// Use db to perform SQL operations on database
	db, err := sql.Open(s.driverName, s.connectStr)
	if err != nil {
		return err
	}
	if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + s.dbName); err != nil {
		return err
	}
	db.Close()

	db, err = sql.Open(s.driverName, s.connectStr+s.dbName+"?autocommit=false&parseTime=true")
	if err != nil {
		return err
	}
	s.db = db
	s.db.SetMaxOpenConns(400)
	s.db.SetMaxIdleConns(10)
	s.db.SetConnMaxLifetime(5 * time.Minute)
	return nil
}

// Stop closes the SQL
func (s *storeBase) Stop(_ context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// Stop closes the SQL
func (s *storeBase) GetDB() *sql.DB {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.db
}

// Transact wrap the transaction
func (s *storeBase) Transact(txFunc func(*sql.Tx) error) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		switch {
		case recover() != nil:
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				logger.Error().Err(rollbackErr) // log err after Rollback
			}
		case err != nil:
			// err is non-nil; don't change it
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				logger.Error().Err(rollbackErr)
			}
		default:
			// err is nil; if Commit returns error update err
			if commitErr := tx.Commit(); commitErr != nil {
				logger.Error().Err(commitErr)
			}
		}
	}()
	err = txFunc(tx)
	return err
}
