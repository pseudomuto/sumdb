package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

// Zip executes a request for the zip file for the specified module. It returns the h1 directory hash of the file.
func (p *Proxy) Zip(ctx context.Context, mod module.Version) (string, error) {
	path, version, err := escapeModule(mod)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"%s/%s/@v/%s.zip",
		p.upstream,
		path,
		version,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed creating zip request: %s, %w", url, err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed reading zip response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get zip, expected: %d, received: %d", http.StatusOK, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "sumdb-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for zip: %w", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write zip file: %w", err)
	}

	h1, err := dirhash.HashZip(f.Name(), dirhash.Hash1)
	if err != nil {
		return "", fmt.Errorf("failed to calculate dirhash for zip: %w", err)
	}

	return h1, nil
}
