package storage_store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// etcdLogger is a package-level logger; no-op by default.
// Inject your service logger via SetEtcdLogger.
var etcdLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))



// Etcd_store is a thin KV wrapper around etcd v3.
type Etcd_store struct {
	client  *clientv3.Client
	address string

	// serialized ops channel (driven by run loop)
	actions chan map[string]interface{}
}

// open connects to etcd. If address is empty, it attempts to read
// "$(configDir)/etcd.yml" and use either "endpoints" (list) or
// "initial-advertise-peer-urls" (string, comma-separated).
func (s *Etcd_store) open(address string) error {
	endpoints, err := s.resolveEndpoints(address)
	if err != nil {
		return err
	}

	fmt.Println("etcd_store.go --------------> Etcd_store: connecting to etcd endpoints:", endpoints) // DEBUG

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}

	s.client = cli
	s.address = strings.Join(endpoints, ",")
	etcdLogger.Info("etcd connected", "endpoints", endpoints)
	return nil
}

func (s *Etcd_store) resolveEndpoints(address string) ([]string, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed != "" {
		return splitCSV(trimmed), nil
	}

	// Fallback: read etcd.yml
	cfgPath := config.GetConfigDir() + "/etcd.yml"
	if cfgPath == "" {
		return nil, errors.New("etcd: config dir not found")
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Prefer an explicit "endpoints" list if present:
	if raw, ok := cfg["endpoints"]; ok {
		switch t := raw.(type) {
		case []interface{}:
			out := make([]string, 0, len(t))
			for _, v := range t {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
			if len(out) > 0 {
				return out, nil
			}
		case []string:
			if len(t) > 0 {
				return t, nil
			}
		}
	}

	// Otherwise accept etcd-style string key:
	if raw, ok := cfg["initial-advertise-peer-urls"]; ok {
		if s, ok2 := raw.(string); ok2 && strings.TrimSpace(s) != "" {
			return splitCSV(s), nil
		}
	}

	return nil, errors.New("etcd: no endpoints found in etcd.yml")
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (s *Etcd_store) setItem(key string, val []byte) error {
	if s.client == nil {
		return errors.New("etcd: setItem on nil client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.client.Put(ctx, key, string(val))
	return err
}

func (s *Etcd_store) getItem(key string) ([]byte, error) {
	if s.client == nil {
		return nil, errors.New("etcd: getItem on nil client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rsp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(rsp.Kvs) == 0 {
		return nil, errors.New("etcd: key not found")
	}
	// If multiple, return the last one (etcd returns sorted by key by default)
	return rsp.Kvs[len(rsp.Kvs)-1].Value, nil
}

func (s *Etcd_store) removeItem(key string) error {
	if s.client == nil {
		return errors.New("etcd: removeItem on nil client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.client.Delete(ctx, key)
	return err
}

func (s *Etcd_store) close() error {
	if s.client == nil {
		return nil // idempotent
	}
	err := s.client.Close()
	s.client = nil
	etcdLogger.Info("etcd closed")
	return err
}
