# sumdb

A custom implementation of the Go sumdb server.

[![CI](https://github.com/pseudomuto/sumdb/actions/workflows/ci.yaml/badge.svg)](https://github.com/pseudomuto/sumdb/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pseudomuto/sumdb.svg)](https://pkg.go.dev/github.com/pseudomuto/sumdb)
[![Go Report Card](https://goreportcard.com/badge/github.com/pseudomuto/sumdb)](https://goreportcard.com/report/github.com/pseudomuto/sumdb)

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
