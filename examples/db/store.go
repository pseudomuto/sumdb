package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pseudomuto/sumdb"
	"golang.org/x/mod/sumdb/tlog"
)

// dbtx abstracts sql.DB and sql.Tx for shared query execution.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// dbStore implements sumdb.Store and sumdb.TxStore using SQLite.
type dbStore struct {
	tx   dbtx    // *sql.DB or *sql.Tx - used for all queries
	db   *sql.DB // original DB - only used by WithTx to start transactions
	skey string
	vkey string
}

// newDBStore creates a new SQLite-backed store.
func newDBStore(ctx context.Context, db *sql.DB) (*dbStore, error) {
	schema := `
		CREATE TABLE records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL,
			version TEXT NOT NULL,
			data BLOB NOT NULL,
			UNIQUE(path, version)
		);

		CREATE TABLE hashes (
			idx INTEGER PRIMARY KEY,
			hash BLOB NOT NULL
		);

		CREATE TABLE tree (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			size INTEGER NOT NULL DEFAULT 0,
			signer_key TEXT NOT NULL,
			verifier_key TEXT NOT NULL
		);
	`
	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	skey, vkey, err := sumdb.GenerateKeys("example.sumdb")
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	if _, err := db.ExecContext(ctx,
		"INSERT INTO tree (id, size, signer_key, verifier_key) VALUES (1, 0, ?, ?)",
		skey, vkey,
	); err != nil {
		return nil, fmt.Errorf("failed to initialize tree: %w", err)
	}

	return &dbStore{
		tx:   db,
		db:   db,
		skey: skey,
		vkey: vkey,
	}, nil
}

// RecordID returns the ID of the record for the given module path and version.
func (s *dbStore) RecordID(ctx context.Context, path, version string) (int64, error) {
	var id int64
	err := s.tx.
		QueryRowContext(ctx, "SELECT id FROM records WHERE path = ? AND version = ?", path, version).
		Scan(&id)
	if err == sql.ErrNoRows {
		return 0, sumdb.ErrNotFound
	}

	if err != nil {
		return 0, fmt.Errorf("query record: %w", err)
	}

	return id, nil
}

// Records returns records with IDs in the interval [id, id+n).
func (s *dbStore) Records(ctx context.Context, id, n int64) ([]*sumdb.Record, error) {
	rows, err := s.tx.QueryContext(ctx,
		"SELECT id, path, version, data FROM records WHERE id >= ? AND id < ? ORDER BY id",
		id, id+n,
	)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []*sumdb.Record
	for rows.Next() {
		r := &sumdb.Record{}
		if err := rows.Scan(&r.ID, &r.Path, &r.Version, &r.Data); err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// AddRecord adds a new entry for the specified module.
func (s *dbStore) AddRecord(ctx context.Context, r *sumdb.Record) (int64, error) {
	res, err := s.tx.ExecContext(ctx,
		"INSERT INTO records (path, version, data) VALUES (?, ?, ?)",
		r.Path, r.Version, r.Data,
	)
	if err != nil {
		return 0, fmt.Errorf("insert record: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get record id after insert: %w", err)
	}

	return id, nil
}

// ReadHashes returns the hashes at the given storage indexes.
func (s *dbStore) ReadHashes(ctx context.Context, indexes []int64) ([]tlog.Hash, error) {
	if len(indexes) == 0 {
		return nil, nil
	}

	// Build a map for ordered retrieval
	result := make([]tlog.Hash, len(indexes))
	indexMap := make(map[int64]int, len(indexes))
	for i, idx := range indexes {
		indexMap[idx] = i
	}

	// Query all hashes at once
	var query strings.Builder
	query.WriteString("SELECT idx, hash FROM hashes WHERE idx IN (")
	args := make([]any, len(indexes))
	for i, idx := range indexes {
		if i > 0 {
			query.WriteString(",")
		}
		query.WriteString("?")
		args[i] = idx
	}
	query.WriteString(")")

	rows, err := s.tx.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query hashes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idx int64
		var hash []byte
		if err := rows.Scan(&idx, &hash); err != nil {
			return nil, fmt.Errorf("scan hash: %w", err)
		}
		if i, ok := indexMap[idx]; ok && len(hash) == tlog.HashSize {
			copy(result[i][:], hash)
		}
	}

	return result, rows.Err()
}

// WriteHashes stores hashes at the given storage indexes.
func (s *dbStore) WriteHashes(ctx context.Context, indexes []int64, hashes []tlog.Hash) error {
	stmt, err := s.tx.PrepareContext(ctx, "INSERT OR REPLACE INTO hashes (idx, hash) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for i, idx := range indexes {
		if _, err := stmt.ExecContext(ctx, idx, hashes[i][:]); err != nil {
			return fmt.Errorf("insert hash at %d: %w", idx, err)
		}
	}

	return nil
}

// TreeSize returns the current number of records in the tree.
func (s *dbStore) TreeSize(ctx context.Context) (int64, error) {
	var size int64
	err := s.tx.
		QueryRowContext(ctx, "SELECT size FROM tree WHERE id = 1").
		Scan(&size)
	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("query tree size: %w", err)
	}

	return size, nil
}

// SetTreeSize updates the tree size.
func (s *dbStore) SetTreeSize(ctx context.Context, size int64) error {
	_, err := s.tx.ExecContext(ctx,
		"UPDATE tree SET size = ? WHERE id = 1",
		size,
	)
	if err != nil {
		return fmt.Errorf("update tree size: %w", err)
	}

	return nil
}

// WithTx implements sumdb.TxStore.
func (s *dbStore) WithTx(ctx context.Context, fn func(sumdb.Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txStore := &dbStore{tx: tx, db: s.db, skey: s.skey, vkey: s.vkey}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
