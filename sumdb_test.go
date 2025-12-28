package sumdb_test

//go:generate go tool mockgen -destination=store_test.go -package=sumdb_test . Store

import (
	"archive/zip"
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	. "github.com/pseudomuto/sumdb"
	"github.com/pseudomuto/sumdb/internal/signer"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/tlog"
)

func TestGenerateKeys(t *testing.T) {
	skey, vkey, err := GenerateKeys("sumdb.example.org")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(skey, "PRIVATE+KEY+sumdb.example.org+"))
	require.True(t, strings.HasPrefix(vkey, "sumdb.example.org+"))
}

func TestSigned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	skey, vkey, err := GenerateKeys("test.example.com")
	require.NoError(t, err)

	store := NewMockStore(ctrl)
	db, err := New("test.example.com", skey, WithStore(store))
	require.NoError(t, err)

	t.Run("empty tree", func(t *testing.T) {
		store.EXPECT().TreeSize(gomock.Any()).Return(int64(0), nil).Times(2)

		signed, err := db.Signed(t.Context())
		require.NoError(t, err)

		verifier, err := signer.NewVerifier(vkey)
		require.NoError(t, err)

		tree, err := signer.VerifyTreeHead(verifier, signed)
		require.NoError(t, err)
		require.Equal(t, int64(0), tree.N)
		require.Equal(t, tlog.Hash{}, tree.Hash)
	})

	t.Run("tree size error", func(t *testing.T) {
		store.EXPECT().TreeSize(gomock.Any()).Return(int64(0), errors.New("db error"))

		_, err = db.Signed(t.Context())
		require.ErrorContains(t, err, "failed to get tree size")
	})

	t.Run("tree hash error", func(t *testing.T) {
		store.EXPECT().TreeSize(gomock.Any()).Return(int64(1), nil).Times(2)
		store.EXPECT().ReadHashes(gomock.Any(), gomock.Any()).Return(nil, errors.New("hash error"))

		_, err = db.Signed(t.Context())
		require.ErrorContains(t, err, "failed to compute tree hash")
	})
}

func TestReadRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	skey, _, err := GenerateKeys("test.example.com")
	require.NoError(t, err)

	store := NewMockStore(ctrl)
	db, err := New("test.example.com", skey, WithStore(store))
	require.NoError(t, err)

	t.Run("returns record data", func(t *testing.T) {
		records := []*Record{
			{ID: 1, Path: "example.com/foo", Version: "v1.0.0", Data: []byte("record 1")},
			{ID: 2, Path: "example.com/bar", Version: "v2.0.0", Data: []byte("record 2")},
		}
		store.EXPECT().
			Records(gomock.Any(), int64(1), int64(2)).
			Return(records, nil)

		data, err := db.ReadRecords(t.Context(), 1, 2)
		require.NoError(t, err)
		require.Len(t, data, 2)
		require.Equal(t, []byte("record 1"), data[0])
		require.Equal(t, []byte("record 2"), data[1])
	})

	t.Run("store error", func(t *testing.T) {
		store.EXPECT().Records(gomock.Any(), int64(5), int64(3)).Return(nil, errors.New("db error"))

		_, err := db.ReadRecords(t.Context(), 5, 3)
		require.ErrorContains(t, err, "failed to get records")
	})
}

func TestReadTileData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	skey, _, err := GenerateKeys("test.example.com")
	require.NoError(t, err)

	store := NewMockStore(ctrl)
	db, err := New("test.example.com", skey, WithStore(store))
	require.NoError(t, err)

	t.Run("returns tile data", func(t *testing.T) {
		tile := tlog.Tile{H: 8, L: 0, N: 0, W: 2}
		hashes := []tlog.Hash{{1}, {2}}
		store.EXPECT().ReadHashes(gomock.Any(), gomock.Any()).Return(hashes, nil)

		data, err := db.ReadTileData(t.Context(), tile)
		require.NoError(t, err)
		require.Len(t, data, 2*tlog.HashSize)
	})

	t.Run("read error", func(t *testing.T) {
		tile := tlog.Tile{H: 8, L: 0, N: 0, W: 1}
		store.EXPECT().ReadHashes(gomock.Any(), gomock.Any()).Return(nil, errors.New("hash error"))

		_, err := db.ReadTileData(t.Context(), tile)
		require.ErrorContains(t, err, "failed reading tile data")
	})
}

func TestLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	skey, _, err := GenerateKeys("test.example.com")
	require.NoError(t, err)

	store := NewMockStore(ctrl)
	db, err := New("test.example.com", skey, WithStore(store))
	require.NoError(t, err)

	t.Run("record exists", func(t *testing.T) {
		mod := module.Version{Path: "example.com/foo", Version: "v1.0.0"}
		store.EXPECT().RecordID(gomock.Any(), mod.Path, mod.Version).Return(int64(42), nil)

		id, err := db.Lookup(t.Context(), mod)
		require.NoError(t, err)
		require.Equal(t, int64(42), id)
	})

	t.Run("record id error", func(t *testing.T) {
		mod := module.Version{Path: "example.com/bar", Version: "v2.0.0"}
		store.EXPECT().RecordID(gomock.Any(), mod.Path, mod.Version).Return(int64(0), errors.New("db error"))

		_, err := db.Lookup(t.Context(), mod)
		require.ErrorContains(t, err, "failed to find record id")
	})

	t.Run("creates new record", func(t *testing.T) {
		// Create minimal module zip in-memory
		var zipBuf bytes.Buffer
		zw := zip.NewWriter(&zipBuf)
		w, err := zw.Create("example.com/new@v1.0.0/go.mod")
		require.NoError(t, err)
		_, err = w.Write([]byte("module example.com/new\n"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		modContent := []byte("module example.com/new\n")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".mod") {
				_, _ = w.Write(modContent)
			} else if strings.HasSuffix(r.URL.Path, ".zip") {
				_, _ = w.Write(zipBuf.Bytes())
			}
		}))
		defer srv.Close()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		skey, _, err := GenerateKeys("test.example.com")
		require.NoError(t, err)

		store := NewMockStore(ctrl)
		upstream, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db, err := New("test.example.com", skey, WithStore(store), WithUpstream(upstream))
		require.NoError(t, err)

		mod := module.Version{Path: "example.com/new", Version: "v1.0.0"}
		store.EXPECT().RecordID(gomock.Any(), mod.Path, mod.Version).Return(int64(0), ErrNotFound)
		store.EXPECT().AddRecord(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		store.EXPECT().ReadHashes(gomock.Any(), gomock.Any()).Return([]tlog.Hash{}, nil).AnyTimes()
		store.EXPECT().WriteHashes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		store.EXPECT().SetTreeSize(gomock.Any(), int64(1)).Return(nil)

		id, err := db.Lookup(t.Context(), mod)
		require.NoError(t, err)
		require.Equal(t, int64(0), id)
	})
}
