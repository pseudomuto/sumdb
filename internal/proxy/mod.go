package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

// GoMod executes a go.mod request and returns the h1 directory hash of the file.
func (p *Proxy) GoMod(ctx context.Context, mod module.Version) (string, error) {
	path, version, err := escapeModule(mod)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"%s/%s/@v/%s.mod",
		p.upstream,
		path,
		version,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed creating go.mod request: %s, %w", url, err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed reading go.mod response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get go.mod, expected: %d, received: %d", http.StatusOK, resp.StatusCode)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod response body: %w", err)
	}

	h1, err := dirhash.Hash1([]string{"go.mod"}, func(s string) (io.ReadCloser, error) {
		return io.NopCloser(&buf), nil
	})
	if err != nil {
		return "", fmt.Errorf("failed calculating h1 hash for go.mod: %w", err)
	}

	return h1, nil
}
