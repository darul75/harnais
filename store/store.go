package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type RunStore struct {
	db *sql.DB
}

func NewRunStore(dbPath string) (*RunStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &RunStore{db: db}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

func (s *RunStore) Close() error {
	return s.db.Close()
}

func (s *RunStore) DB() *sql.DB {
	return s.db
}
