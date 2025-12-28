package proxy

import (
	"fmt"
	"net/http"

	"golang.org/x/mod/module"
)

type (
	// HTTPClient defines an HTTP client for executing requests.
	HTTPClient interface {
		Do(*http.Request) (*http.Response, error)
	}

	// Proxy defines a client for an upstream Go module proxy.
	// See: https://go.dev/ref/mod#goproxy-protocol
	Proxy struct {
		client   HTTPClient // The HTTPClient to use for executing requests.
		upstream string     // The upstream proxy server (e.g. https://proxy.golang.org)
	}
)

// New creates a new Proxy for querying the supplied upstream.
func New(client HTTPClient, upstream string) *Proxy {
	return &Proxy{
		client:   client,
		upstream: upstream,
	}
}

func escapeModule(mod module.Version) (string, string, error) {
	path, err := module.EscapePath(mod.Path)
	if err != nil {
		return "", "", fmt.Errorf("failed to escape path: %s, %w", mod.Path, err)
	}

	version, err := module.EscapeVersion(mod.Version)
	if err != nil {
		return "", "", fmt.Errorf("failed to escape version: %s, %w", mod.Version, err)
	}

	return path, version, nil
}
