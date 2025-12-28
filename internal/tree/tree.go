// Package tree provides Merkle tree operations for sumdb using tlog.
package tree

import (
	"context"
	"fmt"

	"golang.org/x/mod/sumdb/tlog"
)

// TileHeight is the standard tile height used by sumdb.
// Each tile contains 2^TileHeight = 256 hashes.
const TileHeight = 8

type (
	// HashStore defines the interface for hash storage operations.
	// This is a subset of the main Store interface focused on hash operations.
	HashStore interface {
		ReadHashes(ctx context.Context, indexes []int64) ([]tlog.Hash, error)
		WriteHashes(ctx context.Context, indexes []int64, hashes []tlog.Hash) error
		TreeSize(ctx context.Context) (int64, error)
		SetTreeSize(ctx context.Context, size int64) error
	}

	// hashReader adapts a HashStore to implement tlog.HashReader.
	hashReader struct {
		ctx   context.Context
		store HashStore
	}
)

// AddRecord computes and stores the hashes for a new record at the given ID.
// The caller must ensure that id equals the current tree size (i.e., this is
// an append operation). After successful completion, the tree size is incremented.
func AddRecord(ctx context.Context, store HashStore, id int64, data []byte) error {
	hr := &hashReader{ctx: ctx, store: store}

	// Compute hashes that need to be stored for this record
	hashes, err := tlog.StoredHashes(id, data, hr)
	if err != nil {
		return fmt.Errorf("failed to compute hashes for record %d: %w", id, err)
	}

	// Compute storage indexes for each hash
	indexes := storedHashIndexes(id, len(hashes))
	if len(indexes) != len(hashes) {
		return fmt.Errorf("indexes and hashes length mismatch: %d != %d", len(indexes), len(hashes))
	}

	if len(indexes) == 0 {
		return nil
	}

	// Store the hashes
	if err := store.WriteHashes(ctx, indexes, hashes); err != nil {
		return fmt.Errorf("failed to write hashes for record %d: %w", id, err)
	}

	// Update tree size
	if err := store.SetTreeSize(ctx, id+1); err != nil {
		return fmt.Errorf("failed to update tree size: %w", err)
	}

	return nil
}

// ReadTile reads tile data from the store.
// This returns the raw bytes for the tile, suitable for serving to clients.
func ReadTile(ctx context.Context, store HashStore, t tlog.Tile) ([]byte, error) {
	hr := &hashReader{ctx: ctx, store: store}
	data, err := tlog.ReadTileData(t, hr)
	if err != nil {
		return nil, fmt.Errorf("failed to read tile %s: %w", t.Path(), err)
	}
	return data, nil
}

// ReadHashes implements tlog.HashReader.
func (r *hashReader) ReadHashes(indexes []int64) ([]tlog.Hash, error) {
	return r.store.ReadHashes(r.ctx, indexes)
}

// storedHashIndexes computes the storage indexes for hashes produced by
// tlog.StoredHashes(id, data, hr).
//
// When adding record id, StoredHashes returns 1 + (trailing 1-bits in id) hashes:
//   - Hash 0: leaf hash at (level=0, n=id)
//   - Hash k (for k >= 1): subtree root at (level=k, n=id>>k)
//
// Example for id=7 (binary 111): 4 hashes at (0,7), (1,3), (2,1), (3,0)
func storedHashIndexes(id int64, count int) []int64 {
	indexes := make([]int64, count)
	for i := range count {
		indexes[i] = tlog.StoredHashIndex(i, id>>i)
	}
	return indexes
}

// TreeHash returns the current root hash of the tree.
func TreeHash(ctx context.Context, store HashStore) (tlog.Hash, error) {
	size, err := store.TreeSize(ctx)
	if err != nil {
		return tlog.Hash{}, fmt.Errorf("failed to get tree size: %w", err)
	}

	if size == 0 {
		// Empty tree has a well-defined hash
		return tlog.Hash{}, nil
	}

	hr := &hashReader{ctx: ctx, store: store}
	hash, err := tlog.TreeHash(size, hr)
	if err != nil {
		return tlog.Hash{}, fmt.Errorf("failed to compute tree hash: %w", err)
	}

	return hash, nil
}
