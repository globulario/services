package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/plan/versionutil"
)

// ServiceKey uniquely identifies a service by publisher and name.
// Canonical form: "<publisher_id>/<service_name>" with a sanitized service name.
type ServiceKey struct {
	PublisherID string
	ServiceName string
}

func (k ServiceKey) String() string {
	pub := strings.TrimSpace(k.PublisherID)
	if pub == "" {
		pub = "unknown"
	}
	return pub + "/" + canonicalServiceName(k.ServiceName)
}

// InstalledServiceInfo captures what the node knows about an installed service.
type InstalledServiceInfo struct {
	PublisherID string
	ServiceName string
	Version     string
	Config      map[string]string
}

// ComputeInstalledServices returns the installed services on this node and a deterministic hash.
// The hash is stable across runs and independent of map/directory iteration order.
func ComputeInstalledServices(ctx context.Context) (map[ServiceKey]InstalledServiceInfo, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	byService := map[string]*InstalledServiceInfo{}
	var firstErr error
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	loadMarkers(ctx, byService, recordErr)
	loadServiceConfigs(ctx, byService, recordErr)

	inst := make(map[ServiceKey]InstalledServiceInfo, len(byService))
	for _, entry := range byService {
		if entry == nil {
			continue
		}
		if entry.ServiceName == "" || entry.Version == "" {
			continue
		}
		if entry.PublisherID == "" {
			entry.PublisherID = "unknown"
		}
		key := ServiceKey{PublisherID: entry.PublisherID, ServiceName: entry.ServiceName}
		inst[key] = InstalledServiceInfo{
			PublisherID: entry.PublisherID,
			ServiceName: entry.ServiceName,
			Version:     entry.Version,
			Config:      entry.Config,
		}
	}

	hash := computeAppliedServicesHash(inst)
	return inst, hash, firstErr
}

func loadMarkers(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	markerRoot := versionutil.BaseDir()
	entries, err := os.ReadDir(markerRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			recordErr(fmt.Errorf("list version markers: %w", err))
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			recordErr(err)
			return
		}
		path := filepath.Join(markerRoot, name, "version")
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			recordErr(fmt.Errorf("read version marker %s: %w", path, err))
			continue
		}
		version := strings.TrimSpace(string(data))
		if version == "" {
			continue
		}
		svc := canonicalServiceName(name)
		entry := ensureServiceEntry(byService, svc)
		entry.ServiceName = svc
		entry.Version = version
	}
}

func loadServiceConfigs(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	cfgRoot := config.GetServicesConfigDir()
	entries, err := os.ReadDir(cfgRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			recordErr(fmt.Errorf("list service configs: %w", err))
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			recordErr(err)
			return
		}
		path := filepath.Join(cfgRoot, name)
		info, err := os.Stat(path)
		if err != nil {
			recordErr(fmt.Errorf("stat %s: %w", path, err))
			continue
		}
		if info.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			recordErr(fmt.Errorf("read service config %s: %w", path, err))
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			recordErr(fmt.Errorf("parse service config %s: %w", path, err))
			continue
		}
		svc := canonicalServiceName(extractString(raw, "Name", "ServiceName", "service_name", "service"))
		if svc == "" {
			continue
		}
		entry := ensureServiceEntry(byService, svc)
		if entry.ServiceName == "" {
			entry.ServiceName = svc
		}
		if entry.Version == "" {
			if v := extractString(raw, "Version", "version"); v != "" {
				entry.Version = v
			}
		}
		if entry.PublisherID == "" {
			entry.PublisherID = extractString(raw, "PublisherID", "publisher_id", "PublisherId", "publisherId", "Publisher")
		}
		if len(entry.Config) == 0 {
			if cfg := extractStringMap(raw, "Config", "config"); len(cfg) > 0 {
				entry.Config = cfg
			}
		}
	}
}

func computeAppliedServicesHash(installed map[ServiceKey]InstalledServiceInfo) string {
	if len(installed) == 0 {
		return ""
	}
	keys := make([]ServiceKey, 0, len(installed))
	for k := range installed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	var b strings.Builder
	for _, k := range keys {
		info := installed[k]
		b.WriteString("publisher=")
		b.WriteString(strings.TrimSpace(info.PublisherID))
		b.WriteString(";service=")
		b.WriteString(canonicalServiceName(info.ServiceName))
		b.WriteString(";version=")
		b.WriteString(strings.TrimSpace(info.Version))

		cfgKeys := make([]string, 0, len(info.Config))
		for ck := range info.Config {
			cfgKeys = append(cfgKeys, ck)
		}
		sort.Strings(cfgKeys)
		for _, ck := range cfgKeys {
			b.WriteString(";")
			b.WriteString(ck)
			b.WriteString("=")
			b.WriteString(info.Config[ck])
		}
		b.WriteString("|")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func ensureServiceEntry(byService map[string]*InstalledServiceInfo, service string) *InstalledServiceInfo {
	if entry := byService[service]; entry != nil {
		return entry
	}
	entry := &InstalledServiceInfo{ServiceName: service}
	byService[service] = entry
	return entry
}

func canonicalServiceName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimPrefix(n, "globular-")
	n = strings.TrimSuffix(n, ".service")
	return n
}

func extractString(raw map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func extractStringMap(raw map[string]interface{}, keys ...string) map[string]string {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			if m, ok := v.(map[string]interface{}); ok {
				out := make(map[string]string, len(m))
				for mk, mv := range m {
					out[mk] = fmt.Sprint(mv)
				}
				return out
			}
		}
	}
	return nil
}
