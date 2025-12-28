package sumdb

import (
	"fmt"
	"net/http"
	"net/url"
)

// Option configures a SumDB instance.
type Option func(*SumDB)

// WithHTTPClient sets the client used to communicate with the proxy.
func WithHTTPClient(c *http.Client) Option {
	return func(sd *SumDB) { sd.http = c }
}

// WithStore sets the Store for handling persistence of the tree.
func WithStore(s Store) Option {
	return func(sd *SumDB) { sd.store = s }
}

// WithUpstream sets the upstream proxy to query when no records are found.
func WithUpstream(u *url.URL) Option {
	return func(sd *SumDB) {
		sd.upstream = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
}
