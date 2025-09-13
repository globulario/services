package pki

import (
	"context"
	"log/slog"
	"time"
)

// Manager issues/validates/renews node and service certs.
type Manager interface {
	// For nodes/peers that need IP SANs (local CA path)
	EnsurePeerCert(dir string, subject string, dns []string, ips []string, ttl time.Duration) (key, crt, ca string, err error)
	// For public-facing servers (ACME if enabled; else local CA)
	EnsureServerCert(dir string, subject string, dns []string, ttl time.Duration) (key, crt, ca string, err error)
	// Obtain a public ACME certificate for the given DNS names, writing:
	//   <base>.key, <base>.csr, <base>.crt (leaf), <base>.issuer.crt (issuer), <base>.fullchain.pem (leaf+issuer)
	// Does NOT affect server.crt/server.key which remain local-CA for mTLS.
	EnsurePublicACMECert(dir, base, subject string, dns []string, ttl time.Duration) (key, leaf, issuer, fullchain string, err error)

	// For mTLS clients
	EnsureClientCert(dir string, subject string, dns []string, ttl time.Duration) (key, crt, ca string, err error)
	// Validations / rotation
	ValidateCertPair(certFile, keyFile string, requireEKUs []int, requireDNS []string, requireIPs []string) error
	RotateIfExpiring(dir string, leafFile string, renewBefore time.Duration) (rotated bool, err error)

	// Keep parity with globule bootstrapping: write server.key + server.csr with SANs.
	EnsureServerKeyAndCSR(dir, commonName, country, state, city, org string, dns []string) error

}

// ACMEConfig controls Let's Encrypt usage. If disabled, local CA is used.
type ACMEConfig struct {
	Enabled   bool
	Email     string
	Domain    string // primary domain for DNS-01 challenges
	Directory string // ""/prod or "staging"
	Provider  string // "globular", "cloudflare", "" (http-01)
	DNS       string // address of your DNS service when Provider="globular"
	Timeout   time.Duration
}

// LocalCAConfig controls local CA issuance.
type LocalCAConfig struct {
	Enabled   bool
	Password  string
	Country   string
	State     string
	City      string
	Org       string
	ValidDays int
}

// Options configures a FileManager instance.
type Options struct {
	Storage FileStorage
	ACME    ACMEConfig
	LocalCA LocalCAConfig
	Logger  *slog.Logger
	// TokenSource is required when ACME.Provider == "globular"
	// It must return a bearer token to call your DNS API.
	TokenSource func(ctx context.Context, dnsAddr string) (string, error)
}
