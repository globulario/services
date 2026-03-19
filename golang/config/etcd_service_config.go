package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	Utility "github.com/globulario/utility"
)

var runtimeKeys = map[string]struct{}{
	"Process":      {},
	"ProxyProcess": {},
	"State":        {},
	"LastError":    {},
	"ModTime":      {}, // ignore in desired
}

func splitDesiredRuntime(s map[string]interface{}) (desired, runtime map[string]interface{}) {
	desired = make(map[string]interface{}, len(s))
	runtime = map[string]interface{}{
		"UpdatedAt": time.Now().Unix(),
	}
	for k, v := range s {
		if _, ok := runtimeKeys[k]; ok {
			if k != "ModTime" {
				runtime[k] = v
			}
			continue
		}
		desired[k] = v
	}
	if _, ok := desired["Id"]; !ok && s["Id"] != nil {
		desired["Id"] = s["Id"]
	}
	return
}

func mergeDesiredRuntime(desired, runtime map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range desired {
		out[k] = v
	}
	for k, v := range runtime {
		out[k] = v
	}
	if out["Process"] == nil {
		out["Process"] = -1
	}
	if out["ProxyProcess"] == nil {
		out["ProxyProcess"] = -1
	}
	if out["State"] == nil {
		out["State"] = "stopped"
	}
	return out
}

const (
	etcdPrefix          = "/globular/services/"
	configKey           = "config"
	runtimeKey          = "runtime"
	liveKey             = "live"
	globularRootPrefix  = "/globular/"
	etcdSnapshotDirName = "etcd-snapshots"
	servicesBackupName  = "globular_config_backup.json"
)

func etcdKey(id, leaf string) string {
	return etcdPrefix + id + "/" + leaf
}

// BootstrapServicesFromFiles loads all JSON configs from
// the services config directory (default: /var/lib/globular/services)
// and applies them to etcd by calling SaveServiceConfiguration for each.
//
// This only runs when etcd contains NO service configs (cold recovery after
// data-dir loss). On normal startup etcd is authoritative and the disk
// JSON files are just a backup mirror — re-asserting them unconditionally
// would resurrect services that were intentionally removed.
func BootstrapServicesFromFiles() error {
	// Check if etcd already has service configs — if so, etcd is
	// authoritative and we must not overwrite it from disk.
	existing, err := GetServicesConfigurations()
	if err != nil {
		// etcd unavailable — skip bootstrap, caller will retry later.
		fmt.Printf("BootstrapServicesFromFiles: etcd unavailable, skipping: %v\n", err)
		return nil
	}
	if len(existing) > 0 {
		fmt.Printf("BootstrapServicesFromFiles: etcd already has %d service configs, skipping disk bootstrap\n", len(existing))
		return nil
	}

	dir := GetServicesConfigDir()

	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// nothing to bootstrap yet
			return nil
		}
		return fmt.Errorf("bootstrap: stat %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("bootstrap: readdir %s: %w", dir, err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}

		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("BootstrapServicesFromFiles: read %s: %v\n", path, err)
			continue
		}

		var desired map[string]interface{}
		if err := json.Unmarshal(b, &desired); err != nil {
			fmt.Printf("BootstrapServicesFromFiles: unmarshal %s: %v\n", path, err)
			continue
		}

		id := Utility.ToString(desired["Id"])
		if id == "" {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			id = base
			desired["Id"] = id
		}

		if err := SaveServiceConfiguration(desired); err != nil {
			fmt.Printf("BootstrapServicesFromFiles: SaveServiceConfiguration(%s): %v\n", id, err)
			continue
		}
		count++
	}

	fmt.Printf("BootstrapServicesFromFiles: recovered %d service configs from %s to etcd\n", count, dir)
	return nil
}

// SaveServiceConfiguration persists desired/runtime in separate keys
// and mirrors the desired config to a JSON file on disk:
//
//	<ServicesConfigDir>/<Id>.json  (default: /var/lib/globular/services/<Id>.json)
func SaveServiceConfiguration(s map[string]interface{}) error {
	id := Utility.ToString(s["Id"])
	if id == "" {
		return errors.New("SaveServiceConfiguration: missing Id")
	}
	c, err := etcdClient()
	if err != nil {
		return err
	}

	desired, runtime := splitDesiredRuntime(s)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	desBytes, _ := json.MarshalIndent(desired, "", "  ")
	if _, err = c.Put(ctx, etcdKey(id, configKey), string(desBytes)); err != nil {
		return fmt.Errorf("save desired: %w", err)
	}

	rtBytes, _ := json.Marshal(runtime)
	if _, err = c.Put(ctx, etcdKey(id, runtimeKey), string(rtBytes)); err != nil {
		return fmt.Errorf("save runtime: %w", err)
	}

	// Mirror desired config to disk (best effort).
	if err := saveServiceConfigFile(id, desired); err != nil {
		fmt.Printf("SaveServiceConfiguration: failed to persist %s to disk: %v\n", id, err)
	}

	// Fire-and-forget backup of /globular keys (best-effort).
	go func() {
		_, _ = BackupGlobularKeysJSON()
	}()

	return nil
}

// DeleteServiceConfiguration removes a service's config, runtime, and live keys
// from etcd and deletes the on-disk JSON file. Call this when a service is fully
// uninstalled so it no longer appears in the admin catalog.
func DeleteServiceConfiguration(id string) error {
	if id == "" {
		return errors.New("DeleteServiceConfiguration: missing id")
	}
	c, err := etcdClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Delete all keys under /globular/services/<id>/
	prefix := etcdPrefix + id + "/"
	if _, err := c.Delete(ctx, prefix, clientv3.WithPrefix()); err != nil {
		return fmt.Errorf("delete service config from etcd: %w", err)
	}

	// Remove on-disk JSON file (best effort)
	configDir := GetServicesConfigDir()
	filePath := filepath.Join(configDir, id+".json")
	os.Remove(filePath)

	return nil
}

// DeleteServiceConfigurationByName finds and deletes all service config entries
// whose Name field matches the given name (case-insensitive). This is used
// during service removal when only the package name is known, not the service ID.
func DeleteServiceConfigurationByName(name string) error {
	all, err := GetServicesConfigurations()
	if err != nil {
		return err
	}
	for _, s := range all {
		svcName := Utility.ToString(s["Name"])
		if strings.EqualFold(svcName, name) {
			id := Utility.ToString(s["Id"])
			if id != "" {
				if err := DeleteServiceConfiguration(id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GetServicesConfigurations lists and merges all services under /globular/services/.
func GetServicesConfigurations() ([]map[string]interface{}, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Get(ctx, etcdPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	desiredByID := map[string]map[string]interface{}{}
	runtimeByID := map[string]map[string]interface{}{}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.HasPrefix(key, etcdPrefix) {
			continue
		}
		rest := strings.TrimPrefix(key, etcdPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			continue
		}
		id, leaf := parts[0], parts[1]

		switch leaf {
		case configKey:
			var d map[string]interface{}
			if err := json.Unmarshal(kv.Value, &d); err != nil {
				continue
			}
			desiredByID[id] = d
		case runtimeKey:
			var r map[string]interface{}
			if err := json.Unmarshal(kv.Value, &r); err != nil {
				continue
			}
			runtimeByID[id] = r
		}
	}

	var out []map[string]interface{}
	for id, d := range desiredByID {
		r := runtimeByID[id]
		if r == nil {
			r = map[string]interface{}{}
		}
		m := mergeDesiredRuntime(d, r)
		out = append(out, m)
	}
	return out, nil
}

// GetServiceConfigurationById resolves by exact Id, then by Name among all services.
func GetServiceConfigurationById(idOrName string) (map[string]interface{}, error) {
	// Fast path: try exact etcd key lookup (single key, no scan).
	if cfg, err := GetServiceConfigurationByExactId(idOrName); err == nil {
		return cfg, nil
	}

	// Fallback: scan and match by Name
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if Utility.ToString(s["Id"]) == idOrName || strings.EqualFold(Utility.ToString(s["Name"]), idOrName) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no service found with id/name %q", idOrName)
}

// GetServiceConfigurationByExactId does a direct etcd key lookup by exact Id.
// Returns (nil, error) if the service doesn't exist or etcd is unavailable.
// This is much faster than GetServiceConfigurationById because it never falls
// back to scanning all services — use it when you know the exact Id.
func GetServiceConfigurationByExactId(id string) (map[string]interface{}, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	dres, err := c.Get(ctx, etcdKey(id, configKey))
	if err != nil {
		return nil, fmt.Errorf("etcd get config: %w", err)
	}
	if len(dres.Kvs) == 0 {
		return nil, fmt.Errorf("no service found with id %q", id)
	}

	var d map[string]interface{}
	if err := json.Unmarshal(dres.Kvs[0].Value, &d); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	var r map[string]interface{}
	if rres, err := c.Get(ctx, etcdKey(id, runtimeKey)); err == nil && len(rres.Kvs) == 1 {
		_ = json.Unmarshal(rres.Kvs[0].Value, &r)
	}
	return mergeDesiredRuntime(d, r), nil
}

// Plural by-name
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	for _, s := range all {
		if strings.EqualFold(Utility.ToString(s["Name"]), name) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no services found with name %s", name)
	}
	return out, nil
}

func nonEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// Singular by-name: choose the "best" candidate.
func GetServiceConfigurationsByName(name string) (map[string]interface{}, error) {
	candidates, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no services found with name %s", name)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	type ver struct{ major, minor, patch int }
	parseVer := func(v any) ver {
		s := strings.TrimSpace(Utility.ToString(v))
		if s == "" {
			return ver{}
		}
		parts := strings.Split(s, ".")
		out := ver{}
		if len(parts) > 0 {
			out.major, _ = strconv.Atoi(nonEmpty(parts[0]))
		}
		if len(parts) > 1 {
			out.minor, _ = strconv.Atoi(nonEmpty(parts[1]))
		}
		if len(parts) > 2 {
			out.patch, _ = strconv.Atoi(nonEmpty(parts[2]))
		}
		return out
	}

	getUpdatedAt := func(m map[string]interface{}) int64 {
		switch v := m["UpdatedAt"].(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
		return 0
	}

	isRunning := func(m map[string]interface{}) bool {
		return strings.EqualFold(Utility.ToString(m["State"]), "running")
	}

	hasPort := func(m map[string]interface{}) bool {
		return Utility.ToInt(m["Port"]) > 0
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if isRunning(a) != isRunning(b) {
			return isRunning(a)
		}
		ua, ub := getUpdatedAt(a), getUpdatedAt(b)
		if ua != ub {
			return ua > ub
		}
		va, vb := parseVer(a["Version"]), parseVer(b["Version"])
		if va.major != vb.major {
			return va.major > vb.major
		}
		if va.minor != vb.minor {
			return va.minor > vb.minor
		}
		if va.patch != vb.patch {
			return va.patch > vb.patch
		}
		return hasPort(a) && !hasPort(b)
	})
	return candidates[0], nil
}
