# sumdb

A custom implementation of the Go sumdb server.

[![CI](https://github.com/pseudomuto/sumdb/actions/workflows/ci.yaml/badge.svg)](https://github.com/pseudomuto/sumdb/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pseudomuto/sumdb.svg)](https://pkg.go.dev/github.com/pseudomuto/sumdb)
[![Go Report Card](https://goreportcard.com/badge/github.com/pseudomuto/sumdb)](https://goreportcard.com/report/github.com/pseudomuto/sumdb)
[![Coverage](https://codecov.io/gh/pseudomuto/sumdb/branch/main/graph/badge.svg)](https://codecov.io/gh/pseudomuto/sumdb)

This package implements the standard [sumdb protocol](https://go.dev/ref/mod#checksum-database) via
`golang.org/x/mod/sumdb`. When a client requests a module checksum, the server checks the local Store first. If no
record exists, it fetches the module from the upstream proxy (default: proxy.golang.org), computes the h1 hashes, stores
the record with its Merkle tree hashes, and returns the result.

## Usage

```bash
go get github.com/pseudomuto/sumdb
```

Implement the `Store` interface to provide persistence:

- `RecordID` / `Records` / `AddRecord` - module record storage
- `ReadHashes` / `WriteHashes` - Merkle tree hash storage
- `TreeSize` / `SetTreeSize` - tree state management

See [godoc](https://pkg.go.dev/github.com/pseudomuto/sumdb#Store) for the full interface and
[examples/db/](examples/db/) for a complete SQLite implementation.

## Data Model

The sumdb maintains three types of data:

**Records** are module checksum entries. Each record contains the module path, version, and the `h1:` hash lines (one
for the module zip, one for go.mod). Records are assigned sequential IDs starting from 0.

**Hashes** form a [Merkle tree](https://research.swtch.com/tlog) that provides cryptographic proof of the record
history. When a record is added, its content is hashed and incorporated into the tree. The tree structure allows clients
to verify that records haven't been tampered with and that the server is append-only.

**Tree size** tracks the current number of records. This is used to compute the tree's root hash and to determine where
new records are inserted.

When a new module is looked up:

1. The module is fetched from the upstream proxy and its `h1:` hashes are computed
2. A record is created with the module's checksums
3. The Merkle tree hashes are computed for the new record's position
4. The tree size is incremented

The signed tree head (returned by `Signed()`) contains the current tree size and root hash, signed with the server's
private key. Clients use this to verify the integrity of records they receive.

## Concurrency

The `SumDB` type is safe for concurrent use. Module lookups use a three-tier concurrency model:

1. **Fast path (concurrent)**: Existing records are looked up via `RecordID` without any locking. Multiple goroutines can
   read simultaneously.

2. **Singleflight deduplication**: When a module isn't found, concurrent requests for the _same_ module are deduplicated.
   Only one goroutine fetches from the upstream proxy; others wait and receive the same result. This prevents redundant
   network calls.

3. **Serialized writes**: Record creation is protected by a mutex because each record's position in the Merkle tree
   depends on the current tree size. Concurrent inserts of _different_ modules are serialized to maintain tree
   consistency.

If your `Store` implementation supports transactions (by implementing `TxStore`), the record insert and tree hash
updates are wrapped in a transaction for atomicity. This ensures that a failure during tree hash computation won't leave
an orphaned record.

**Important**: A `Store` instance should only be used by a single `SumDB`. Sharing a `Store` across multiple `SumDB`
instances is not supported and may corrupt the Merkle tree.
