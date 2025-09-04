
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/StalkR/httpcache"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36"
	cacheTTL  = 24 * time.Hour
)

// customTransport implements http.RoundTripper interface to add some headers.
type customTransport struct {
	http.RoundTripper
}

func (e *customTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Accept-Language", "en") // avoid IP-based language detection
	r.Header.Set("User-Agent", userAgent)
	return e.RoundTripper.RoundTrip(r)
}

// httpClient is used by requests to perform cached requests.
// If cache directory exists it is used as a persistent cache.
// Otherwise a volatile memory cache is used.
var httpClient *http.Client

// getHTTPClient returns a singleton HTTP client with caching and sane defaults.
func getHTTPClient() *http.Client {
	if httpClient != nil {
		return httpClient
	}

	if _, err := os.Stat("cache"); err == nil {
		var err error
		httpClient, err = httpcache.NewPersistentClient("cache", cacheTTL)
		if err != nil {
			logger.Error("http cache create failed", "err", err)
			panic(err)
		}
	} else {
		httpClient = httpcache.NewVolatileClient(cacheTTL, 1024)
	}

	httpClient.Transport = &customTransport{httpClient.Transport}
	return httpClient
}
