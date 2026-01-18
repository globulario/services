// Package testutil provides shared utilities for service client tests.
// It allows tests to run in different environments by supporting
// environment variable configuration with sensible defaults.
package testutil

import (
	"os"
	"testing"
)

const (
	// Default test configuration values
	DefaultDomain     = "localhost"
	DefaultAddress    = "localhost:8080"
	DefaultSAUser     = "sa"
	DefaultSAPassword = "adminadmin"
)

// Environment variable names
const (
	EnvDomain       = "GLOBULAR_DOMAIN"
	EnvAddress      = "GLOBULAR_ADDRESS"
	EnvSAUser       = "GLOBULAR_SA_USER"
	EnvSAPassword   = "GLOBULAR_SA_PWD"
	EnvTLSCertPath  = "GLOBULAR_TLS_CERT"
	EnvTLSKeyPath   = "GLOBULAR_TLS_KEY"
	EnvTLSCAPath    = "GLOBULAR_TLS_CA"
	EnvSkipExternal = "GLOBULAR_SKIP_EXTERNAL_TESTS"
)

// GetDomain returns the test domain from environment or default.
func GetDomain() string {
	if v := os.Getenv(EnvDomain); v != "" {
		return v
	}
	return DefaultDomain
}

// GetAddress returns the test service address from environment or default.
func GetAddress() string {
	if v := os.Getenv(EnvAddress); v != "" {
		return v
	}
	// Fall back to domain if address not set
	if v := os.Getenv(EnvDomain); v != "" {
		return v
	}
	return DefaultAddress
}

// GetSACredentials returns the super-admin username and password.
func GetSACredentials() (username, password string) {
	username = os.Getenv(EnvSAUser)
	if username == "" {
		username = DefaultSAUser
	}
	password = os.Getenv(EnvSAPassword)
	if password == "" {
		password = DefaultSAPassword
	}
	return
}

// GetTLSPaths returns TLS certificate paths from environment.
// Returns empty strings if not configured.
func GetTLSPaths() (certPath, keyPath, caPath string) {
	return os.Getenv(EnvTLSCertPath), os.Getenv(EnvTLSKeyPath), os.Getenv(EnvTLSCAPath)
}

// SkipIfNoExternalServices skips the test if external services are not available.
// Set GLOBULAR_SKIP_EXTERNAL_TESTS=false to run these tests.
func SkipIfNoExternalServices(t *testing.T) {
	t.Helper()
	skipEnv := os.Getenv(EnvSkipExternal)
	// Default to skipping unless explicitly set to "false" or "0"
	if skipEnv != "false" && skipEnv != "0" {
		t.Skip("Skipping test that requires external services. Set GLOBULAR_SKIP_EXTERNAL_TESTS=false to run.")
	}
}

// SkipIfNoTLS skips the test if TLS is not configured.
func SkipIfNoTLS(t *testing.T) {
	t.Helper()
	cert, key, _ := GetTLSPaths()
	if cert == "" || key == "" {
		t.Skip("Skipping TLS test. Set GLOBULAR_TLS_CERT and GLOBULAR_TLS_KEY to run.")
	}
}

// RequireEnv checks that the specified environment variable is set,
// otherwise skips the test.
func RequireEnv(t *testing.T, envVar string) string {
	t.Helper()
	v := os.Getenv(envVar)
	if v == "" {
		t.Skipf("Skipping test: %s not set", envVar)
	}
	return v
}

// GetEnvOrDefault returns the environment variable value or the default.
func GetEnvOrDefault(envVar, defaultVal string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultVal
}
