// Command db demonstrates using sumdb with SQLite storage.
//
// This example creates an in-memory database, generates signing keys,
// looks up some modules from the Go module proxy, and displays the
// resulting tree state.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/pseudomuto/sumdb"
	"golang.org/x/mod/module"

	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Creating database ===")
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store, err := newDBStore(ctx, db)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Println("Created database")
	fmt.Println()
	fmt.Println("Verification key: " + store.vkey)
	fmt.Println()

	// Create sumdb instance
	sdb, err := sumdb.New("example", store.skey,
		sumdb.WithStore(store),
	)
	if err != nil {
		log.Fatalf("failed to create SumDB: %v", err)
	}

	// 4. Look up some modules
	fmt.Println("\n=== Looking up modules ===")
	modules := []string{
		"github.com/pseudomuto/protoc-gen-doc@v1.5.1",
		"github.com/stretchr/testify@v1.8.0",
		"golang.org/x/mod@v0.17.0",
	}

	for _, mod := range modules {
		m, err := parseModule(mod)
		if err != nil {
			log.Fatalf("parse module: %v", err)
		}

		fmt.Printf("Looking up %s...\n", mod)
		id, err := sdb.Lookup(ctx, m)
		if err != nil {
			log.Fatalf("lookup %s: %v", mod, err)
		}
		fmt.Printf("  → Record ID: %d\n", id)
	}

	// 5. Show tree state
	fmt.Println("\n=== Signed tree head ===")
	signed, err := sdb.Signed(ctx)
	if err != nil {
		log.Fatalf("get signed tree: %v", err)
	}
	fmt.Print(string(signed))

	// 6. Show records
	fmt.Println("\n=== Records in database ===")
	size, err := store.TreeSize(ctx)
	if err != nil {
		log.Fatalf("get tree size: %v", err)
	}

	records, err := store.Records(ctx, 0, size)
	if err != nil {
		log.Fatalf("get records: %v", err)
	}

	for _, r := range records {
		fmt.Printf("[%d] %s@%s\n", r.ID, r.Path, r.Version)
		// Show the checksum data indented
		for line := range strings.SplitSeq(strings.TrimSpace(string(r.Data)), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
}

// parseModule parses a module@version string.
func parseModule(s string) (module.Version, error) {
	parts := strings.SplitN(s, "@", 2)
	if len(parts) != 2 {
		return module.Version{}, fmt.Errorf("invalid module format: %q (expected path@version)", s)
	}
	return module.Version{
		Path:    parts[0],
		Version: parts[1],
	}, nil
}
