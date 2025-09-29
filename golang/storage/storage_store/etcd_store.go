package storage_store

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
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

	cli, err :=  config.GetEtcdClient()
	if err != nil {
		return err
	}
	
	s.client = cli
	s.address = address
	return nil
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

// Get all keys is not supported by etcd KV.
func (s *Etcd_store) getAllKeys() ([]string, error) {
	// Get the list of keys by querying with an empty key and WithKeysOnly option
	if s.client == nil {
		return nil, errors.New("etcd: getAllKeys on nil client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rsp, err := s.client.Get(ctx, "", clientv3.WithFromKey(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(rsp.Kvs))
	for _, kv := range rsp.Kvs {
		keys = append(keys, string(kv.Key))
	}

	return keys, nil
}