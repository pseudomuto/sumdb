package proxy_test

import (
	"path/filepath"
	"testing"

	. "github.com/pseudomuto/sumdb/internal/proxy"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

func TestProxy_Zip(t *testing.T) {
	r, err := recorder.New(filepath.Join("testdata", "zip"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Stop()) })

	proxy := New(r.GetDefaultClient(), "https://proxy.golang.org")

	t.Run("valid request", func(t *testing.T) {
		h1, err := proxy.Zip(t.Context(), module.Version{
			Path:    "github.com/pseudomuto/protoc-gen-doc",
			Version: "v1.5.1",
		})

		require.NoError(t, err)
		require.Equal(t, "h1:Ah259kcrio7Ix1Rhb6u8FCaOkzf9qRBqXnvAufg061w=", h1)
	})

	t.Run("invalid version", func(t *testing.T) {
		_, err := proxy.Zip(t.Context(), module.Version{
			Path:    "github.com/pseudomuto/protoc-gen-doc",
			Version: "1.5.1",
		})

		require.ErrorContains(t, err, "404")
	})
}
