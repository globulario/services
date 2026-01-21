package certs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type CertBundle struct {
	Key        []byte
	Fullchain  []byte
	CA         []byte
	Generation uint64
	UpdatedMS  int64
}

type KV interface {
	AcquireCertIssuerLock(ctx context.Context, domain, nodeID string, ttl time.Duration) (bool, func(), error)
	PutBundle(ctx context.Context, domain string, bundle CertBundle) error
	GetBundle(ctx context.Context, domain string) (CertBundle, error)
	WaitForBundle(ctx context.Context, domain string, timeout time.Duration) (CertBundle, error)
	GetBundleGeneration(ctx context.Context, domain string) (uint64, error)
}

type etcdKV struct {
	client *clientv3.Client
}

func NewEtcdKV(c *clientv3.Client) KV {
	if c == nil {
		return nil
	}
	return &etcdKV{client: c}
}

func lockKey(domain string) string   { return fmt.Sprintf("/globular/pki/locks/%s", domain) }
func bundleKey(domain string) string { return fmt.Sprintf("/globular/pki/bundles/%s", domain) }

func (kv *etcdKV) AcquireCertIssuerLock(ctx context.Context, domain, nodeID string, ttl time.Duration) (bool, func(), error) {
	if kv == nil || kv.client == nil {
		return false, nil, fmt.Errorf("etcd client unavailable")
	}
	leaseResp, err := kv.client.Grant(ctx, int64(ttl.Seconds()))
	if err != nil {
		return false, nil, err
	}
	key := lockKey(domain)
	txn := kv.client.Txn(ctx).If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, nodeID, clientv3.WithLease(leaseResp.ID)))
	resp, err := txn.Commit()
	if err != nil {
		return false, nil, err
	}
	if !resp.Succeeded {
		kv.client.Revoke(ctx, leaseResp.ID)
		return false, nil, nil
	}
	kaCtx, cancel := context.WithCancel(context.Background())
	ch, err := kv.client.KeepAlive(kaCtx, leaseResp.ID)
	if err != nil {
		cancel()
		return false, nil, err
	}
	go func() {
		for range ch {
		}
	}()
	release := func() {
		cancel()
		kv.client.Revoke(context.Background(), leaseResp.ID)
	}
	return true, release, nil
}

type bundlePayload struct {
	Generation uint64 `json:"generation"`
	UpdatedMS  int64  `json:"updated_ms"`
	Key        string `json:"key"`
	Fullchain  string `json:"fullchain"`
	CA         string `json:"ca"`
}

func (kv *etcdKV) PutBundle(ctx context.Context, domain string, bundle CertBundle) error {
	if kv == nil || kv.client == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	current, _ := kv.GetBundle(ctx, domain)
	gen := current.Generation + 1
	if bundle.Generation > gen {
		gen = bundle.Generation
	}
	payload := bundlePayload{
		Generation: gen,
		UpdatedMS:  time.Now().UnixMilli(),
		Key:        base64.StdEncoding.EncodeToString(bundle.Key),
		Fullchain:  base64.StdEncoding.EncodeToString(bundle.Fullchain),
		CA:         base64.StdEncoding.EncodeToString(bundle.CA),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = kv.client.Put(ctx, bundleKey(domain), string(data))
	return err
}

func (kv *etcdKV) GetBundle(ctx context.Context, domain string) (CertBundle, error) {
	if kv == nil || kv.client == nil {
		return CertBundle{}, fmt.Errorf("etcd client unavailable")
	}
	resp, err := kv.client.Get(ctx, bundleKey(domain))
	if err != nil {
		return CertBundle{}, err
	}
	if len(resp.Kvs) == 0 {
		return CertBundle{}, fmt.Errorf("bundle not found")
	}
	var payload bundlePayload
	if err := json.Unmarshal(resp.Kvs[0].Value, &payload); err != nil {
		return CertBundle{}, err
	}
	bundle := CertBundle{
		Generation: payload.Generation,
		UpdatedMS:  payload.UpdatedMS,
	}
	if bundle.Key, err = base64.StdEncoding.DecodeString(payload.Key); err != nil {
		return CertBundle{}, err
	}
	if bundle.Fullchain, err = base64.StdEncoding.DecodeString(payload.Fullchain); err != nil {
		return CertBundle{}, err
	}
	if payload.CA != "" {
		if bundle.CA, err = base64.StdEncoding.DecodeString(payload.CA); err != nil {
			return CertBundle{}, err
		}
	}
	return bundle, nil
}

func (kv *etcdKV) WaitForBundle(ctx context.Context, domain string, timeout time.Duration) (CertBundle, error) {
	deadline := time.Now().Add(timeout)
	for {
		bundle, err := kv.GetBundle(ctx, domain)
		if err == nil {
			return bundle, nil
		}
		if time.Now().After(deadline) {
			return CertBundle{}, fmt.Errorf("bundle not found for %s within timeout", domain)
		}
		time.Sleep(time.Second)
	}
}

func (kv *etcdKV) GetBundleGeneration(ctx context.Context, domain string) (uint64, error) {
	bundle, err := kv.GetBundle(ctx, domain)
	if err != nil {
		return 0, err
	}
	return bundle.Generation, nil
}
