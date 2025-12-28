package sumdb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pseudomuto/sumdb/internal/proxy"
	"github.com/pseudomuto/sumdb/internal/signer"
	"github.com/pseudomuto/sumdb/internal/tree"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/note"
	"golang.org/x/mod/sumdb/tlog"
	"golang.org/x/sync/singleflight"
)

// SumDB is a checksum database server that implements the Go sumdb protocol.
//
// It implements the ServerOpts interface defined in https://pkg.go.dev/golang.org/x/mod@v0.31.0/sumdb#ServerOps.
type SumDB struct {
	http     *http.Client
	proxy    *proxy.Proxy
	store    Store
	signer   note.Signer
	upstream string

	// Used to dedupe proxy calls
	lookupGroup singleflight.Group
}

// New creates a new SumDB instance with the given server name and signing key.
// The name identifies this sumdb (e.g., "sum.example.com").
// The skey must be in note signer format: "PRIVATE+KEY+<name>+<hash>+<keydata>".
//
// NB: You can use GenerateKeys to create a valid signing key.
func New(name string, skey string, opts ...Option) (*SumDB, error) {
	db := &SumDB{
		http: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
			},
		},
		upstream: "https://proxy.golang.org",
	}
	for _, opt := range opts {
		opt(db)
	}

	s, err := signer.NewSigner(skey)
	if err != nil {
		return nil, fmt.Errorf("invalid signer key: %w", err)
	}

	db.proxy = proxy.New(db.http, db.upstream)
	db.signer = s
	return db, nil
}

// GenerateKeys creates a new keypair and returns the encoded signer key,
// and verifier key.
//
// The name identifies the key (e.g., "sum.golang.org").
//
// The signer key is secret and must be stored securely.
// The verifier key can be shared publicly for clients to verify signatures.
func GenerateKeys(name string) (string, string, error) {
	skey, vkey, err := note.GenerateKey(rand.Reader, name)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate key: %w", err)
	}

	return skey, vkey, nil
}

// Handler returns an HTTP handler for serving the sumdb over HTTP.
func (s *SumDB) Handler() http.Handler {
	return sumdb.NewServer(s)
}

// Signed returns the signed tree head for the current tree state.
func (s *SumDB) Signed(ctx context.Context) ([]byte, error) {
	size, err := s.store.TreeSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree size: %w", err)
	}

	hash, err := tree.TreeHash(ctx, s.store)
	if err != nil {
		return nil, fmt.Errorf("failed to compute tree hash: %w", err)
	}

	t := tlog.Tree{N: size, Hash: hash}
	signed, err := signer.SignTreeHead(s.signer, t)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tree head: %w", err)
	}

	return signed, nil
}

// ReadRecords returns the raw data for records with IDs in [id, id+n).
func (s *SumDB) ReadRecords(ctx context.Context, id, n int64) ([][]byte, error) {
	recs, err := s.store.Records(ctx, id, n)
	if err != nil {
		return nil, fmt.Errorf("failed to get records: [%d, %d), %w", id, n, err)
	}

	data := make([][]byte, len(recs))
	for i := range recs {
		data[i] = recs[i].Data
	}

	return data, nil
}

// Lookup finds or creates a record for the given module version.
// If the record doesn't exist, it fetches the module from the upstream proxy,
// computes the checksums, and stores the new record with its tree hashes.
// Concurrent lookups for the same module are deduplicated via singleflight.
func (s *SumDB) Lookup(ctx context.Context, mod module.Version) (int64, error) {
	// Fast path - record already exists
	id, err := s.store.RecordID(ctx, mod.Path, mod.Version)
	if err == nil {
		return id, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return 0, fmt.Errorf("failed to find record id: %w", err)
	}

	// Use singleflight to deduplicate concurrent lookups for the same module
	key := mod.Path + "@" + mod.Version
	result, err, _ := s.lookupGroup.Do(key, func() (any, error) {
		return s.fetchAndStoreRecord(ctx, mod)
	})
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// fetchAndStoreRecord fetches a module from upstream, computes checksums,
// and stores the record. Called via singleflight to deduplicate concurrent requests.
func (s *SumDB) fetchAndStoreRecord(ctx context.Context, mod module.Version) (int64, error) {
	// Double-check: another request may have added it while we waited
	id, err := s.store.RecordID(ctx, mod.Path, mod.Version)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return 0, fmt.Errorf("failed to find record id: %w", err)
	}

	h1mod, err := s.proxy.GoMod(ctx, mod)
	if err != nil {
		return 0, fmt.Errorf("failed getting h1 hash for go.mod: %s, %w", mod.String(), err)
	}

	h1, err := s.proxy.Zip(ctx, mod)
	if err != nil {
		return 0, fmt.Errorf("failed getting h1 hash for module zip: %s, %w", mod.String(), err)
	}

	rec := &Record{
		Path:    mod.Path,
		Version: mod.Version,
		Data: fmt.Appendf(nil,
			"%s %s %s\n%s %s/go.mod %s\n",
			mod.Path,
			mod.Version,
			h1,
			mod.Path,
			mod.Version,
			h1mod,
		),
	}

	id, err = s.store.AddRecord(ctx, rec)
	if err != nil {
		return 0, fmt.Errorf("failed to add new record: %s, %w", mod, err)
	}

	// Compute and store tree hashes for this record
	if err := tree.AddRecord(ctx, s.store, id, rec.Data); err != nil {
		return 0, fmt.Errorf("failed to update tree hashes: %s, %w", mod, err)
	}

	return id, nil
}

// ReadTileData returns the raw record data for a data tile.
// Data tiles (L=-1) contain concatenated record data rather than hashes.
func (s *SumDB) ReadTileData(ctx context.Context, t tlog.Tile) ([]byte, error) {
	data, err := tree.ReadTile(ctx, s.store, t)
	if err != nil {
		return nil, fmt.Errorf("failed reading tile data: %w", err)
	}

	return data, nil
}
