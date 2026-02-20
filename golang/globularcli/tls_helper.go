package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc/credentials"

	"github.com/globulario/services/golang/config"
)

// ErrNeedInstallCerts is returned when mTLS credentials are required but missing.
// Callers should surface this error with the embedded message so users know
// how to fix the problem.
var ErrNeedInstallCerts = errors.New("mTLS client credentials required; run 'globular auth install-certs'")

// clientKeypair holds resolved paths to the user's client certificate and key.
type clientKeypair struct {
	certFile string
	keyFile  string
}

// userGlobularDir returns the user's Globular config directory (~/.config/globular).
// It uses HOME or os.UserHomeDir() and never falls back to system paths.
func userGlobularDir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
	}
	return filepath.Join(home, ".config", "globular"), nil
}

// userPKIPath returns the absolute path to a named file inside the user's PKI directory
// (~/.config/globular/pki/<name>).
func userPKIPath(name string) (string, error) {
	dir, err := userGlobularDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pki", name), nil
}

// resolveCAPath returns the path to the first readable CA certificate.
//
// Priority order:
//  1. GLOBULAR_CA_CERT environment variable
//  2. ~/.config/globular/pki/ca.crt
//  3. config.GetCACertificatePath() (canonical system CA at /var/lib/globular/pki/ca.crt)
//
// Permission errors are never silently ignored.
func resolveCAPath() (string, error) {
	// Priority 1: explicit env var override
	if v := os.Getenv("GLOBULAR_CA_CERT"); v != "" {
		if _, err := os.Stat(v); err != nil {
			if os.IsPermission(err) {
				return "", fmt.Errorf("GLOBULAR_CA_CERT: permission denied: %s", v)
			}
			return "", fmt.Errorf("GLOBULAR_CA_CERT: %w", err)
		}
		return v, nil
	}

	// Priority 2: user PKI directory
	if p, err := userPKIPath("ca.crt"); err == nil {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		} else if os.IsPermission(statErr) {
			return "", fmt.Errorf("CA certificate: permission denied: %s", p)
		}
	}

	// Priority 3: legacy per-domain CA location used by Day-0 installer
	// (~/.config/globular/tls/<domain>/ca.crt).  Try common domain names so
	// that 'auth login' works before the user has run 'auth install-certs'.
	if globularDir, gErr := userGlobularDir(); gErr == nil {
		legacyTLSDir := filepath.Join(globularDir, "tls")
		for _, domain := range []string{"localhost", "globular.internal"} {
			p := filepath.Join(legacyTLSDir, domain, "ca.crt")
			if _, statErr := os.Stat(p); statErr == nil {
				return p, nil
			} else if os.IsPermission(statErr) {
				return "", fmt.Errorf("CA certificate: permission denied: %s", p)
			}
		}
		// Also accept any ca.crt found one level under tls/ (covers custom domains).
		if entries, readErr := os.ReadDir(legacyTLSDir); readErr == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				p := filepath.Join(legacyTLSDir, e.Name(), "ca.crt")
				if _, statErr := os.Stat(p); statErr == nil {
					return p, nil
				}
			}
		}
	}

	// Priority 4: canonical system CA path
	if caPath := config.GetCACertificatePath(); caPath != "" {
		if _, err := os.Stat(caPath); err == nil {
			return caPath, nil
		}
	}

	return "", fmt.Errorf("CA certificate not found (set GLOBULAR_CA_CERT or run 'globular auth install-certs')")
}

// resolveClientKeypair returns the resolved user client certificate and key paths.
//
// Priority order:
//  1. GLOBULAR_CLIENT_CERT and GLOBULAR_CLIENT_KEY environment variables (must both be set)
//  2. ~/.config/globular/pki/client.crt and ~/.config/globular/pki/client.key
//
// Permission errors are never collapsed into "not found" – they are returned
// as explicit errors so the caller can surface the right diagnostic message.
//
// If requireClientCert is true and no keypair can be located, ErrNeedInstallCerts
// is returned. If false, nil is returned (caller proceeds without client cert).
func resolveClientKeypair(requireClientCert bool) (*clientKeypair, error) {
	// Priority 1: explicit env vars
	certEnv := os.Getenv("GLOBULAR_CLIENT_CERT")
	keyEnv := os.Getenv("GLOBULAR_CLIENT_KEY")
	if certEnv != "" || keyEnv != "" {
		if certEnv == "" || keyEnv == "" {
			return nil, errors.New("GLOBULAR_CLIENT_CERT and GLOBULAR_CLIENT_KEY must both be set")
		}
		if _, err := os.Stat(certEnv); err != nil {
			if os.IsPermission(err) {
				return nil, fmt.Errorf("GLOBULAR_CLIENT_CERT: permission denied: %s", certEnv)
			}
			return nil, fmt.Errorf("GLOBULAR_CLIENT_CERT: %w", err)
		}
		if _, err := os.Stat(keyEnv); err != nil {
			if os.IsPermission(err) {
				return nil, fmt.Errorf("GLOBULAR_CLIENT_KEY: permission denied: %s", keyEnv)
			}
			return nil, fmt.Errorf("GLOBULAR_CLIENT_KEY: %w", err)
		}
		return &clientKeypair{certFile: certEnv, keyFile: keyEnv}, nil
	}

	// Priority 2: user PKI directory
	certPath, cerr := userPKIPath("client.crt")
	keyPath, kerr := userPKIPath("client.key")
	if cerr == nil && kerr == nil {
		certStatErr := func() error { _, e := os.Stat(certPath); return e }()
		keyStatErr := func() error { _, e := os.Stat(keyPath); return e }()

		// Surface permission errors explicitly – never collapse to "not found"
		if certStatErr != nil && os.IsPermission(certStatErr) {
			return nil, fmt.Errorf("client cert: permission denied: %s", certPath)
		}
		if keyStatErr != nil && os.IsPermission(keyStatErr) {
			return nil, fmt.Errorf("client key: permission denied: %s", keyPath)
		}

		// Both files present and readable
		if certStatErr == nil && keyStatErr == nil {
			return &clientKeypair{certFile: certPath, keyFile: keyPath}, nil
		}
	}

	// Priority 3: legacy per-domain TLS path (~/.config/globular/tls/<domain>/)
	// Written by generate-user-client-cert.sh during Day-0 installation.
	// Try common domain names so that 'pkg publish' works before the user
	// has run 'auth install-certs' (which writes to the pki/ dir).
	if globularDir, gErr := userGlobularDir(); gErr == nil {
		legacyTLSDir := filepath.Join(globularDir, "tls")
		// Collect domains: known names first, then any dir under tls/
		domains := []string{"localhost", "globular.internal"}
		if entries, readErr := os.ReadDir(legacyTLSDir); readErr == nil {
			for _, e := range entries {
				if e.IsDir() {
					domains = append(domains, e.Name())
				}
			}
		}
		seen := make(map[string]struct{})
		for _, domain := range domains {
			if _, ok := seen[domain]; ok {
				continue
			}
			seen[domain] = struct{}{}
			cPath := filepath.Join(legacyTLSDir, domain, "client.crt")
			kPath := filepath.Join(legacyTLSDir, domain, "client.key")
			certStatErr := func() error { _, e := os.Stat(cPath); return e }()
			keyStatErr := func() error { _, e := os.Stat(kPath); return e }()
			if certStatErr != nil && os.IsPermission(certStatErr) {
				return nil, fmt.Errorf("client cert: permission denied: %s", cPath)
			}
			if keyStatErr != nil && os.IsPermission(keyStatErr) {
				return nil, fmt.Errorf("client key: permission denied: %s", kPath)
			}
			if certStatErr == nil && keyStatErr == nil {
				return &clientKeypair{certFile: cPath, keyFile: kPath}, nil
			}
		}
	}

	// Keypair not found
	if requireClientCert {
		return nil, ErrNeedInstallCerts
	}
	return nil, nil
}

// getTLSCredentials creates gRPC transport credentials with optional mTLS.
// The CA certificate is always required; the client keypair is loaded when
// available and silently omitted when it is not (server-auth-only mode).
//
// Use getTLSCredentialsWithOptions(true) when a client keypair is mandatory.
func getTLSCredentials() (credentials.TransportCredentials, error) {
	return getTLSCredentialsWithOptions(false)
}

// getTLSCredentialsWithOptions creates gRPC transport credentials.
//
// When requireClientCert is true the call fails with ErrNeedInstallCerts if the
// user client keypair cannot be located, and no network connection is attempted.
// When requireClientCert is false the call succeeds even without a client keypair
// (server-auth-only TLS).
func getTLSCredentialsWithOptions(requireClientCert bool) (credentials.TransportCredentials, error) {
	// Resolve CA certificate (always required)
	caPath, err := resolveCAPath()
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(caPath)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("CA certificate: permission denied: %s", caPath)
		}
		return nil, fmt.Errorf("read CA %s: %w", caPath, err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
	}

	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	// Resolve client keypair
	kp, err := resolveClientKeypair(requireClientCert)
	if err != nil {
		return nil, err
	}
	if kp != nil {
		clientCert, err := tls.LoadX509KeyPair(kp.certFile, kp.keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client keypair (%s, %s): %w", kp.certFile, kp.keyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
