package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Tier-0 host lists in etcd — these are the infrastructure components that
// cannot themselves be addressed via cluster DNS (because DNS depends on them).
// Stored as plain JSON arrays of IPv4 addresses. Services read these lists to
// reach DNS / Scylla / etcd without any env vars or hardcoded addresses.
const (
	EtcdKeyClusterDNSHosts    = "/globular/cluster/dns/hosts"    // DNS resolver IPs
	EtcdKeyClusterScyllaHosts = "/globular/cluster/scylla/hosts" // Scylla seed IPs
	EtcdKeyClusterMinioHosts  = "/globular/cluster/minio/hosts"  // MinIO endpoint IPs
)

// LoadClusterHostList reads a JSON IP list from etcd at the given key.
// Returns an error if the key is missing, malformed, or empty. The returned
// slice is a copy — callers may mutate it freely.
//
// Rejects any entry containing "127.0.0.1" or "localhost" — loopback is never
// a valid cluster address (rule: etcd/DNS only, no loopback ever).
func LoadClusterHostList(key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("cluster hosts: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("cluster hosts: etcd get %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("cluster hosts: %s not set in etcd", key)
	}
	var hosts []string
	if err := json.Unmarshal(resp.Kvs[0].Value, &hosts); err != nil {
		return nil, fmt.Errorf("cluster hosts: parse %s: %w", key, err)
	}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if strings.Contains(h, "127.0.0.1") || strings.EqualFold(h, "localhost") {
			return nil, fmt.Errorf("cluster hosts: %s contains loopback entry %q — refuse to use", key, h)
		}
		out = append(out, h)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cluster hosts: %s is empty", key)
	}
	return out, nil
}

// SaveClusterHostList writes a JSON IP list to etcd at the given key.
// Rejects loopback entries at write time.
func SaveClusterHostList(key string, hosts []string) error {
	cleaned := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if strings.Contains(h, "127.0.0.1") || strings.EqualFold(h, "localhost") {
			return fmt.Errorf("cluster hosts: refuse to write loopback %q to %s", h, key)
		}
		cleaned = append(cleaned, h)
	}
	if len(cleaned) == 0 {
		return fmt.Errorf("cluster hosts: %s would be empty", key)
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		return fmt.Errorf("cluster hosts: marshal: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("cluster hosts: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("cluster hosts: etcd put %s: %w", key, err)
	}
	return nil
}

// GetDNSHosts returns the list of Globular DNS resolver IPs (Tier-0).
// Services use these IPs to dial the DNS daemon — DNS cannot itself be
// resolved via DNS.
func GetDNSHosts() ([]string, error) {
	return LoadClusterHostList(EtcdKeyClusterDNSHosts)
}

// GetScyllaHosts returns the list of Scylla seed IPs (Tier-0).
// DNS stores its records in Scylla, so Scylla cannot be addressed via DNS.
func GetScyllaHosts() ([]string, error) {
	return LoadClusterHostList(EtcdKeyClusterScyllaHosts)
}

// GetMinioHosts returns the list of MinIO endpoint IPs (Tier-0).
// The repository and backup-manager use these IPs to reach MinIO without
// depending on DNS resolution (MinIO starts before DNS).
func GetMinioHosts() ([]string, error) {
	return LoadClusterHostList(EtcdKeyClusterMinioHosts)
}
