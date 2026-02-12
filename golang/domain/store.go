package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/dnsprovider"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	DomainSpecPrefix     = "/globular/domains/v1/"
	DomainStatusSuffix   = "/status"
	ProviderConfigPrefix = "/globular/dns/providers/"
)

// DomainStore provides storage operations for domain specs and status, with separation
// to prevent concurrent updates from overwriting user intent.
type DomainStore interface {
	// Spec operations (user intent)
	ListSpecs(ctx context.Context) ([]*ExternalDomainSpec, error)
	GetSpec(ctx context.Context, fqdn string) (*ExternalDomainSpec, int64, error) // returns modRevision
	PutSpec(ctx context.Context, spec *ExternalDomainSpec) error
	DeleteSpec(ctx context.Context, fqdn string) error

	// Status operations (controller-owned)
	GetStatus(ctx context.Context, fqdn string) (*ExternalDomainStatus, int64, error) // returns modRevision
	PutStatus(ctx context.Context, fqdn string, status *ExternalDomainStatus) error
	PutStatusCAS(ctx context.Context, fqdn string, status *ExternalDomainStatus, expectModRev int64) (bool, error)

	// Provider config operations
	ListProviderConfigs(ctx context.Context) ([]*dnsprovider.Config, error)
	GetProviderConfig(ctx context.Context, ref string) (*dnsprovider.Config, int64, error)
}

// EtcdDomainStore implements DomainStore using etcd as the backend.
type EtcdDomainStore struct {
	cli *clientv3.Client
}

// NewEtcdDomainStore creates a new EtcdDomainStore.
func NewEtcdDomainStore(cli *clientv3.Client) *EtcdDomainStore {
	return &EtcdDomainStore{cli: cli}
}

// DomainSpecKey returns the etcd key for a domain spec.
func DomainSpecKey(fqdn string) string {
	return DomainSpecPrefix + fqdn
}

// DomainStatusKey returns the etcd key for a domain status.
func DomainStatusKey(fqdn string) string {
	return DomainSpecPrefix + fqdn + DomainStatusSuffix
}

// ProviderConfigKey returns the etcd key for a provider config.
func ProviderConfigKey(ref string) string {
	return ProviderConfigPrefix + ref
}

// ListSpecs returns all domain specs.
func (s *EtcdDomainStore) ListSpecs(ctx context.Context) ([]*ExternalDomainSpec, error) {
	resp, err := s.cli.Get(ctx, DomainSpecPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get failed: %w", err)
	}

	var specs []*ExternalDomainSpec
	for _, kv := range resp.Kvs {
		// Skip status keys
		if strings.HasSuffix(string(kv.Key), DomainStatusSuffix) {
			continue
		}

		var spec ExternalDomainSpec
		if err := json.Unmarshal(kv.Value, &spec); err != nil {
			// Log but don't fail the entire list
			continue
		}
		specs = append(specs, &spec)
	}

	return specs, nil
}

// GetSpec retrieves a domain spec and its modification revision.
func (s *EtcdDomainStore) GetSpec(ctx context.Context, fqdn string) (*ExternalDomainSpec, int64, error) {
	key := DomainSpecKey(fqdn)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return nil, 0, fmt.Errorf("etcd get failed: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, 0, fmt.Errorf("spec not found: %s", fqdn)
	}

	var spec ExternalDomainSpec
	if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
		return nil, 0, fmt.Errorf("unmarshal spec failed: %w", err)
	}

	return &spec, resp.Kvs[0].ModRevision, nil
}

// PutSpec writes a domain spec.
func (s *EtcdDomainStore) PutSpec(ctx context.Context, spec *ExternalDomainSpec) error {
	key := DomainSpecKey(spec.FQDN)
	data, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal spec failed: %w", err)
	}

	_, err = s.cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put failed: %w", err)
	}

	return nil
}

// DeleteSpec removes a domain spec.
func (s *EtcdDomainStore) DeleteSpec(ctx context.Context, fqdn string) error {
	key := DomainSpecKey(fqdn)
	_, err := s.cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd delete failed: %w", err)
	}
	return nil
}

// GetStatus retrieves a domain status and its modification revision.
func (s *EtcdDomainStore) GetStatus(ctx context.Context, fqdn string) (*ExternalDomainStatus, int64, error) {
	key := DomainStatusKey(fqdn)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return nil, 0, fmt.Errorf("etcd get failed: %w", err)
	}
	if len(resp.Kvs) == 0 {
		// Status doesn't exist yet - return empty status with zero revision
		return &ExternalDomainStatus{}, 0, nil
	}

	var status ExternalDomainStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &status); err != nil {
		return nil, 0, fmt.Errorf("unmarshal status failed: %w", err)
	}

	return &status, resp.Kvs[0].ModRevision, nil
}

// PutStatus writes a domain status (unconditionally).
func (s *EtcdDomainStore) PutStatus(ctx context.Context, fqdn string, status *ExternalDomainStatus) error {
	key := DomainStatusKey(fqdn)
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status failed: %w", err)
	}

	_, err = s.cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put failed: %w", err)
	}

	return nil
}

// PutStatusCAS writes a domain status with compare-and-swap to prevent concurrent overwrites.
// Returns (true, nil) if write succeeded, (false, nil) if revision mismatch, or error on failure.
func (s *EtcdDomainStore) PutStatusCAS(ctx context.Context, fqdn string, status *ExternalDomainStatus, expectModRev int64) (bool, error) {
	key := DomainStatusKey(fqdn)
	data, err := json.Marshal(status)
	if err != nil {
		return false, fmt.Errorf("marshal status failed: %w", err)
	}

	var cmp clientv3.Cmp
	if expectModRev == 0 {
		// Expect key to not exist
		cmp = clientv3.Compare(clientv3.Version(key), "=", 0)
	} else {
		// Expect specific revision
		cmp = clientv3.Compare(clientv3.ModRevision(key), "=", expectModRev)
	}

	txn := s.cli.Txn(ctx).If(cmp).Then(clientv3.OpPut(key, string(data)))
	resp, err := txn.Commit()
	if err != nil {
		return false, fmt.Errorf("etcd txn failed: %w", err)
	}

	return resp.Succeeded, nil
}

// ListProviderConfigs returns all DNS provider configurations.
func (s *EtcdDomainStore) ListProviderConfigs(ctx context.Context) ([]*dnsprovider.Config, error) {
	resp, err := s.cli.Get(ctx, ProviderConfigPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get failed: %w", err)
	}

	var configs []*dnsprovider.Config
	for _, kv := range resp.Kvs {
		var cfg dnsprovider.Config
		if err := json.Unmarshal(kv.Value, &cfg); err != nil {
			// Log but don't fail the entire list
			continue
		}
		configs = append(configs, &cfg)
	}

	return configs, nil
}

// GetProviderConfig retrieves a provider config and its modification revision.
func (s *EtcdDomainStore) GetProviderConfig(ctx context.Context, ref string) (*dnsprovider.Config, int64, error) {
	key := ProviderConfigKey(ref)
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return nil, 0, fmt.Errorf("etcd get failed: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, 0, fmt.Errorf("provider config not found: %s", ref)
	}

	var cfg dnsprovider.Config
	if err := json.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, 0, fmt.Errorf("unmarshal provider config failed: %w", err)
	}

	return &cfg, resp.Kvs[0].ModRevision, nil
}
