package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	Utility "github.com/globulario/utility"
)

// CreateEtcdSnapshot saves a binary etcd snapshot under
//
//	<configDir>/etcd-snapshots/etcd-<unix>.db
//
// and returns the snapshot filepath.
//
// You still need to use "etcdutl snapshot restore" offline to rebuild
// a corrupted data-dir from this file, but this gives you the snapshot.
func CreateEtcdSnapshot() (string, error) {
	c, err := etcdClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	m := clientv3.NewMaintenance(c)
	r, err := m.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	defer r.Close()

	snapDir := filepath.Join(GetConfigDir(), etcdSnapshotDirName)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", fmt.Errorf("snapshot mkdir: %w", err)
	}

	fn := filepath.Join(snapDir, fmt.Sprintf("etcd-%d.db", time.Now().Unix()))
	f, err := os.Create(fn)
	if err != nil {
		return "", fmt.Errorf("snapshot create: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("snapshot copy: %w", err)
	}
	return fn, nil
}

// BackupGlobularKeysJSON exports all keys under "/globular/" (including
// /globular/services, /globular/accounts, etc.) to a JSON file:
//
//	<configDir>/backups/globular_config_backup.json
//
// It returns the full path to the backup file.
func BackupGlobularKeysJSON() (string, error) {
	c, err := etcdClient()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.Get(ctx, globularRootPrefix, clientv3.WithPrefix())
	if err != nil {
		return "", fmt.Errorf("backup etcd get: %w", err)
	}

	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	payload := struct {
		CreatedAt int64 `json:"created_at"`
		Items     []kv  `json:"items"`
	}{
		CreatedAt: time.Now().Unix(),
		Items:     make([]kv, 0, len(resp.Kvs)),
	}
	for _, kvp := range resp.Kvs {
		payload.Items = append(payload.Items, kv{
			Key:   string(kvp.Key),
			Value: string(kvp.Value),
		})
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("backup marshal: %w", err)
	}

	backupDir := filepath.Join(GetConfigDir(), "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("backup mkdir: %w", err)
	}

	tmp := filepath.Join(backupDir, servicesBackupName+".tmp")
	final := filepath.Join(backupDir, servicesBackupName)

	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return "", fmt.Errorf("backup write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", fmt.Errorf("backup rename: %w", err)
	}
	return final, nil
}

// RestoreGlobularKeysJSON replays all keys from a JSON backup file created
// by BackupGlobularKeysJSON into the current etcd cluster.
//
// Use this AFTER you have rebuilt or re-initialized your etcd data-dir.
func RestoreGlobularKeysJSON(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("restore read file: %w", err)
	}

	var payload struct {
		CreatedAt int64 `json:"created_at"`
		Items     []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return fmt.Errorf("restore unmarshal: %w", err)
	}

	c, err := etcdClient()
	if err != nil {
		return fmt.Errorf("restore etcd connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, kv := range payload.Items {
		if _, err := c.Put(ctx, kv.Key, kv.Value); err != nil {
			return fmt.Errorf("restore put %s: %w", kv.Key, err)
		}
	}
	return nil
}

// saveServiceConfigFile writes the "desired" config to
//
//	<ServicesConfigDir>/<id>.json  (default: /var/lib/globular/services/<id>.json)
//
// using an atomic tmp+rename write.
func saveServiceConfigFile(id string, desired map[string]interface{}) error {
	if id == "" {
		return fmt.Errorf("saveServiceConfigFile: empty id")
	}

	dir := GetServicesConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("saveServiceConfigFile mkdir: %w", err)
	}

	// Ensure Id is present in the payload.
	if _, ok := desired["Id"]; !ok {
		desired["Id"] = id
	}

	b, err := json.MarshalIndent(desired, "", "  ")
	if err != nil {
		return fmt.Errorf("saveServiceConfigFile marshal: %w", err)
	}

	tmp := filepath.Join(dir, id+".json.tmp")
	final := filepath.Join(dir, id+".json")

	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("saveServiceConfigFile write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("saveServiceConfigFile rename: %w", err)
	}
	return nil
}

// DumpServiceConfigsToDisk reads all service configs from etcd and writes
// each one to <ServicesConfigDir>/<id>.json.  This is useful after an etcd
// snapshot restore so that the disk mirror is re-populated before services
// start.  Errors are collected but never fatal.
func DumpServiceConfigsToDisk() (int, []string) {
	cfgs, err := GetServicesConfigurations()
	if err != nil {
		return 0, []string{fmt.Sprintf("list services: %v", err)}
	}

	var errs []string
	count := 0
	for _, cfg := range cfgs {
		id := Utility.ToString(cfg["Id"])
		if id == "" {
			continue
		}
		if err := saveServiceConfigFile(id, cfg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", id, err))
			continue
		}
		count++
	}
	return count, errs
}
