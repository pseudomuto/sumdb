package sumdb

import (
	"context"
	"errors"

	"golang.org/x/mod/sumdb/tlog"
)

// ErrNotFound is returned when a requested record does not exist in the store.
var ErrNotFound = errors.New("record not found")

type (
	// Record represents a module checksum entry in the sumdb.
	Record struct {
		ID      int64
		Path    string
		Version string
		Data    []byte
	}

	// Store defines the persistence interface for sumdb data.
	// Implementations must be safe for concurrent use.
	//
	// A Store instance should only be used by a single SumDB. Sharing a Store
	// across multiple SumDB instances is not supported and may corrupt the
	// Merkle tree, as write serialization is handled at the SumDB level.
	Store interface {
		// RecordID returns the ID of the record for the given module path and version.
		// Returns ErrNotFound if no record exists.
		RecordID(ctx context.Context, path, version string) (int64, error)

		// Records returns records with IDs in the interval [id, id+n).
		// The returned slice may have fewer than n records if the range extends
		// beyond the current tree size.
		Records(ctx context.Context, id, n int64) ([]*Record, error)

		// AddRecord adds a new entry for the specified module.
		// The record's ID field is ignored; the store assigns the next sequential ID.
		// Returns the assigned ID.
		AddRecord(ctx context.Context, r *Record) (int64, error)

		// ReadHashes returns the hashes at the given storage indexes.
		// Indexes are computed using tlog.StoredHashIndex(level, n).
		// The returned slice must have the same length as indexes.
		ReadHashes(ctx context.Context, indexes []int64) ([]tlog.Hash, error)

		// WriteHashes stores hashes at the given storage indexes.
		// indexes and hashes must have the same length.
		WriteHashes(ctx context.Context, indexes []int64, hashes []tlog.Hash) error

		// TreeSize returns the current number of records in the tree.
		TreeSize(ctx context.Context) (int64, error)

		// SetTreeSize updates the tree size.
		// This should be called after successfully adding a record and its hashes.
		SetTreeSize(ctx context.Context, size int64) error
	}

	// TxStore is an optional extension of Store that provides transaction support.
	// When a Store implements TxStore, atomic operations will use transactions.
	//
	// Implementations that do not support transactions can simply implement Store.
	// The SumDB will detect TxStore support at runtime and use transactions when available.
	TxStore interface {
		Store

		// WithTx executes fn within a database transaction.
		// If fn returns nil, the transaction is committed.
		// If fn returns an error or panics, the transaction is rolled back.
		//
		// The Store passed to fn represents the transactional view and must be used
		// for all operations within the callback.
		WithTx(ctx context.Context, fn func(Store) error) error
	}
)
