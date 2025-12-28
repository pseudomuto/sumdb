package tree_test

import (
	"context"
	"testing"

	. "github.com/pseudomuto/sumdb/internal/tree"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/sumdb/tlog"
)

// mockStore implements HashStore for testing.
type mockStore struct {
	hashes   map[int64]tlog.Hash
	treeSize int64
}

func TestAddRecord_SingleRecord(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	data := []byte("github.com/example/foo v1.0.0 h1:abc123\n")

	err := AddRecord(ctx, store, 0, data)
	require.NoError(t, err)

	// Tree size should be incremented
	size, err := store.TreeSize(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), size)

	// Should have stored the record hash
	require.NotEmpty(t, store.hashes)
}

func TestAddRecord_MultipleRecords(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	records := []string{
		"github.com/example/a v1.0.0 h1:aaa\n",
		"github.com/example/b v1.0.0 h1:bbb\n",
		"github.com/example/c v1.0.0 h1:ccc\n",
		"github.com/example/d v1.0.0 h1:ddd\n",
	}

	for i, data := range records {
		err := AddRecord(ctx, store, int64(i), []byte(data))
		require.NoError(t, err)
	}

	// Tree size should match record count
	size, err := store.TreeSize(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(4), size)

	// Should have stored multiple hashes (leaf + internal nodes)
	require.Greater(t, len(store.hashes), 4)
}

func TestTreeHash_EmptyTree(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	hash, err := TreeHash(ctx, store)
	require.NoError(t, err)
	require.Equal(t, tlog.Hash{}, hash)
}

func TestTreeHash_WithRecords(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	// Add some records
	records := []string{
		"github.com/example/a v1.0.0 h1:aaa\n",
		"github.com/example/b v1.0.0 h1:bbb\n",
	}

	for i, data := range records {
		err := AddRecord(ctx, store, int64(i), []byte(data))
		require.NoError(t, err)
	}

	hash, err := TreeHash(ctx, store)
	require.NoError(t, err)

	// Hash should not be empty
	require.NotEqual(t, tlog.Hash{}, hash)
}

func TestTreeHash_Consistency(t *testing.T) {
	ctx := context.Background()

	// Two stores with same data should produce same hash
	store1 := newMockStore()
	store2 := newMockStore()

	data := []byte("github.com/example/foo v1.0.0 h1:abc123\n")

	require.NoError(t, AddRecord(ctx, store1, 0, data))
	require.NoError(t, AddRecord(ctx, store2, 0, data))

	hash1, err := TreeHash(ctx, store1)
	require.NoError(t, err)

	hash2, err := TreeHash(ctx, store2)
	require.NoError(t, err)

	require.Equal(t, hash1, hash2)
}

func TestTreeHash_DifferentData(t *testing.T) {
	ctx := context.Background()

	store1 := newMockStore()
	store2 := newMockStore()

	require.NoError(t, AddRecord(ctx, store1, 0, []byte("data1\n")))
	require.NoError(t, AddRecord(ctx, store2, 0, []byte("data2\n")))

	hash1, err := TreeHash(ctx, store1)
	require.NoError(t, err)

	hash2, err := TreeHash(ctx, store2)
	require.NoError(t, err)

	// Different data should produce different hashes
	require.NotEqual(t, hash1, hash2)
}

func TestReadTile(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	// Add enough records to have a complete tile at level 0
	// With tile height 8, we need 256 records for a complete tile
	// For this test, we'll just add a few and read a partial tile
	for i := range 4 {
		data := []byte("record " + string(rune('0'+i)) + "\n")
		require.NoError(t, AddRecord(ctx, store, int64(i), data))
	}

	// Read tile at level 0, offset 0 (partial tile with 4 records)
	tile := tlog.Tile{H: TileHeight, L: 0, N: 0, W: 4}
	data, err := ReadTile(ctx, store, tile)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Tile data should be W * HashSize bytes
	expectedSize := 4 * tlog.HashSize
	require.Len(t, data, expectedSize)
}

func TestAddRecord_IncrementalTreeHash(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	var prevHash tlog.Hash

	// Add records one by one and verify tree hash changes
	for i := range 8 {
		data := []byte("record " + string(rune('0'+i)) + "\n")
		require.NoError(t, AddRecord(ctx, store, int64(i), data))

		hash, err := TreeHash(ctx, store)
		require.NoError(t, err)

		if i > 0 {
			// Hash should change with each new record
			require.NotEqual(t, prevHash, hash, "hash should change after adding record %d", i)
		}
		prevHash = hash
	}
}

func newMockStore() *mockStore {
	return &mockStore{
		hashes: make(map[int64]tlog.Hash),
	}
}

func (m *mockStore) ReadHashes(_ context.Context, indexes []int64) ([]tlog.Hash, error) {
	result := make([]tlog.Hash, len(indexes))
	for i, idx := range indexes {
		result[i] = m.hashes[idx]
	}
	return result, nil
}

func (m *mockStore) WriteHashes(_ context.Context, indexes []int64, hashes []tlog.Hash) error {
	for i, idx := range indexes {
		m.hashes[idx] = hashes[i]
	}
	return nil
}

func (m *mockStore) TreeSize(_ context.Context) (int64, error) {
	return m.treeSize, nil
}

func (m *mockStore) SetTreeSize(_ context.Context, size int64) error {
	m.treeSize = size
	return nil
}
