package upstream

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A definitive HTTP 404 from an upstream must surface as ErrNotFound so the sync
// pipeline classifies it codes.NotFound (non-retryable absence) rather than
// codes.Unavailable (transient). This is the local-build Day-0 case: a version
// that only exists locally was never published to the upstream, so its tag 404s.

func TestHTTPGet_404ReturnsErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := httpGet(srv.URL+"/v9.9.9/release-index.json", "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 must wrap ErrNotFound, got: %v", err)
	}
}

func TestHTTPGet_5xxIsNotErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := httpGet(srv.URL+"/v1.0.0/release-index.json", "")
	if err == nil {
		t.Fatal("expected error for 502")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("502 is transient and must NOT wrap ErrNotFound, got: %v", err)
	}
}

func TestHTTPOpen_404ReturnsErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _, err := httpOpen(srv.URL+"/missing.tgz", "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 must wrap ErrNotFound, got: %v", err)
	}
}

func TestDoGitHubRequest_404WrapsErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := doGitHubRequest(srv.URL+"/repos/x/y/releases/tags/v9.9.9", "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GitHub API 404 must wrap ErrNotFound, got: %v", err)
	}
}
