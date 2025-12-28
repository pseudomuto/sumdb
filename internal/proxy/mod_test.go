package proxy_test

import (
	"path/filepath"
	"testing"

	. "github.com/pseudomuto/sumdb/internal/proxy"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

func TestProxy_GoMod(t *testing.T) {
	r, err := recorder.New(filepath.Join("testdata", "gomod"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Stop()) })

	proxy := New(r.GetDefaultClient(), "https://proxy.golang.org")

	t.Run("valid request", func(t *testing.T) {
		h1, err := proxy.GoMod(t.Context(), module.Version{
			Path:    "github.com/pseudomuto/protoc-gen-doc",
			Version: "v1.5.1",
		})

		require.NoError(t, err)
		require.Equal(t, "h1:XpMKYg6zkcpgfpCfQ8GcWBDRtRxOmMR5w7pz4Xo+dYM=", h1)
	})

	t.Run("invalid version", func(t *testing.T) {
		_, err := proxy.GoMod(t.Context(), module.Version{
			Path:    "github.com/pseudomuto/protoc-gen-doc",
			Version: "1.5.1",
		})

		require.ErrorContains(t, err, "404")
	})
}
