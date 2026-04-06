package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	liveMu     sync.Mutex
	liveLeases = map[string]*LiveLease{}
)

//go:schemalint:ignore — implementation type, not schema owner
type LiveLease struct {
	LeaseID clientv3.LeaseID
	cancel  context.CancelFunc
}

// Lightweight runtime getters (kept for compatibility).
func runtimeEtcdKey(id string) string { return fmt.Sprintf("/globular/services/%s/runtime", id) }

func GetRuntime(id string) (map[string]any, error) {
	if id == "" {
		return nil, errors.New("GetRuntime: empty id")
	}
	cli, err := etcdClient()
	if err != nil {
		return nil, fmt.Errorf("GetRuntime: etcd connect: %w", err)
	}
	key := runtimeEtcdKey(id)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		resp, err := cli.Get(ctx, key, clientv3.WithSerializable())
		cancel()
		if err == nil {
			if len(resp.Kvs) == 0 {
				nowSec := time.Now().Unix()
				return map[string]any{"Process": -1, "State": "stopped", "LastError": "", "UpdatedAt": nowSec}, nil
			}
			var rt map[string]any
			if uerr := json.Unmarshal(resp.Kvs[0].Value, &rt); uerr != nil {
				return nil, fmt.Errorf("GetRuntime: unmarshal: %w", uerr)
			}
			if _, ok := rt["UpdatedAt"]; !ok {
				rt["UpdatedAt"] = time.Now().Unix()
			}
			return rt, nil
		}
		lastErr = err
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}
	return nil, fmt.Errorf("GetRuntime: etcd get %s: %w", key, lastErr)
}

func PutRuntime(id string, patch map[string]any) error {
	if id == "" {
		return errors.New("PutRuntime: empty id")
	}
	if patch == nil {
		patch = map[string]any{}
	}
	current, err := GetRuntime(id)
	if err != nil || current == nil {
		current = map[string]any{}
	}
	for k, v := range patch {
		current[k] = v
	}
	current["UpdatedAt"] = time.Now().Unix()

	b, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("PutRuntime: marshal: %w", err)
	}
	cli, err := etcdClient()
	if err != nil {
		return fmt.Errorf("PutRuntime: etcd connect: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		_, err = cli.Put(ctx, runtimeEtcdKey(id), string(b))
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("PutRuntime: etcd put: %w", err)
}

func StartLive(id string, ttlSeconds int64) (*LiveLease, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 15
	}
	lease := clientv3.NewLease(c)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	g, err := lease.Grant(ctx, ttlSeconds)
	if err != nil {
		return nil, err
	}

	if _, err = c.Put(context.Background(), etcdKey(id, liveKey), "", clientv3.WithLease(g.ID)); err != nil {
		_, _ = lease.Revoke(context.Background(), g.ID)
		return nil, err
	}

	kaCtx, kaCancel := context.WithCancel(context.Background())
	ch, err := lease.KeepAlive(kaCtx, g.ID)
	if err != nil {
		kaCancel()
		_, _ = lease.Revoke(context.Background(), g.ID)
		return nil, err
	}
	go func() {
		for range ch {
		}
	}()

	ll := &LiveLease{LeaseID: g.ID, cancel: kaCancel}
	liveMu.Lock()
	liveLeases[id] = ll
	liveMu.Unlock()
	return ll, nil
}

func StopLive(id string) {
	liveMu.Lock()
	ll := liveLeases[id]
	delete(liveLeases, id)
	liveMu.Unlock()

	if ll == nil {
		return
	}
	ll.cancel()
	if c, err := etcdClient(); err == nil {
		_, _ = c.Lease.Revoke(context.Background(), ll.LeaseID)
	}
}

//go:schemalint:ignore — implementation type, not schema owner
type RuntimeEvent struct {
	ID      string
	Runtime map[string]interface{}
}

func WatchRuntimes(ctx context.Context, cb func(RuntimeEvent)) error {
	c, err := etcdClient()
	if err != nil {
		return err
	}

	wch := c.Watch(ctx, etcdPrefix, clientv3.WithPrefix())
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case wr, ok := <-wch:
			if !ok {
				return errors.New("etcd watch channel closed")
			}
			for _, ev := range wr.Events {
				if ev.Kv == nil {
					continue
				}
				key := string(ev.Kv.Key) // /globular/services/<id>/runtime
				if !strings.HasPrefix(key, etcdPrefix) || !strings.HasSuffix(key, "/"+runtimeKey) {
					continue
				}
				rest := strings.TrimPrefix(key, etcdPrefix)
				parts := strings.SplitN(rest, "/", 2)
				if len(parts) != 2 {
					continue
				}
				id := parts[0]
				var rt map[string]interface{}
				if err := json.Unmarshal(ev.Kv.Value, &rt); err != nil {
					continue
				}
				cb(RuntimeEvent{ID: id, Runtime: rt})
			}
		}
	}
}
