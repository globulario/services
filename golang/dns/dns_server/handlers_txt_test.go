package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/globulario/services/golang/dns/dnspb"
)

// stubStore is a minimal storage_store.Store whose GetItem always returns the
// configured bytes and error, allowing handler tests to run without ScyllaDB.
type stubStore struct {
	data []byte
	err  error
}

func (s *stubStore) Open(optionsStr string) error      { return nil }
func (s *stubStore) Close() error                      { return nil }
func (s *stubStore) SetItem(key string, val []byte) error { return nil }
func (s *stubStore) GetItem(key string) ([]byte, error)   { return s.data, s.err }
func (s *stubStore) RemoveItem(key string) error          { return nil }
func (s *stubStore) Clear() error                         { return nil }
func (s *stubStore) Drop() error                          { return nil }
func (s *stubStore) GetAllKeys() ([]string, error)        { return nil, nil }

func newTestServer(store *stubStore) *server {
	return &server{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		store:  store,
		// depHealth nil → requireHealthy() returns nil (no gate)
	}
}

// TestGetTXT_EmptyStore_ReturnsEmptySlice is the regression test for the
// "unexpected end of JSON input" panic. ScyllaDB returns ([]byte{}, nil) for
// keys that exist as nil/empty in the store; the handler must treat this as
// "no records" and return an empty slice, not an error.
func TestGetTXT_EmptyStore_ReturnsEmptySlice(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		domain string
	}{
		{"nil bytes", nil, "globular.internal"},
		{"empty bytes", []byte{}, "dns.globular.internal"},
		{"empty bytes api", []byte{}, "api.globular.internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(&stubStore{data: tc.data, err: nil})
			resp, err := s.GetTXT(context.Background(), &dnspb.GetTXTRequest{Domain: tc.domain})
			if err != nil {
				t.Fatalf("GetTXT(%q) with empty store: expected nil error, got %v", tc.domain, err)
			}
			if len(resp.Txt) != 0 {
				t.Fatalf("GetTXT(%q) with empty store: expected empty Txt, got %v", tc.domain, resp.Txt)
			}
		})
	}
}

func TestGetDomains_FallsBackToInMemoryCacheWhenStoreFails(t *testing.T) {
	s := newTestServer(&stubStore{err: assertErr("scylla quorum unavailable")})
	s.Domains = []string{"globular.internal.", "example.internal."}

	resp, err := s.GetDomains(context.Background(), &dnspb.GetDomainsRequest{})
	if err != nil {
		t.Fatalf("GetDomains with store error: expected cache fallback, got err=%v", err)
	}
	if len(resp.GetDomains()) != 2 {
		t.Fatalf("expected 2 cached domains, got %d", len(resp.GetDomains()))
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
