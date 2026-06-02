// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.dnsprovider.noop
// @awareness file_role=null_implementation_of_external_dns_provider_for_test_and_disabled_modes
// @awareness risk=low
package dnsprovider

// noop.go — Provider implementation that logs the would-be call
// and writes nothing. Used by tests and when external DNS is
// intentionally disabled. The log lines are part of the contract:
// operators rely on "would upsert..." appearing in the journal to
// confirm the provider is actually wired up before they flip to
// cloudflare/rfc2136. Removing the logs would break the
// pre-rollout sanity-check workflow.

import (
	"context"
	"log"
	"net"
)

// NoopProvider is a no-op provider for testing or when external DNS is disabled (PR8)
type NoopProvider struct {
	config Config
}

// NewNoopProvider creates a new no-op provider
func NewNoopProvider(cfg Config) *NoopProvider {
	return &NoopProvider{config: cfg}
}

// UpsertA does nothing
func (p *NoopProvider) UpsertA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	log.Printf("external dns (noop): would upsert A record %s -> %v (ttl=%d)", name, ips, ttl)
	return nil
}

// UpsertAAAA does nothing
func (p *NoopProvider) UpsertAAAA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	log.Printf("external dns (noop): would upsert AAAA record %s -> %v (ttl=%d)", name, ips, ttl)
	return nil
}

// Delete does nothing
func (p *NoopProvider) Delete(ctx context.Context, name string) error {
	log.Printf("external dns (noop): would delete record %s", name)
	return nil
}

// Close does nothing
func (p *NoopProvider) Close() error {
	return nil
}
