package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	Utility "github.com/globulario/utility"

	// NEW: zap logger so we can control etcd client verbosity
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// =============================
// Etcd client & conventions
// =============================

var (
	cliOnce sync.Once
	cli     *clientv3.Client
	cliErr  error

	// live leases tracked by supervisor (id -> lease)
	liveMu     sync.Mutex
	liveLeases = map[string]*LiveLease{}
)

type LiveLease struct {
	LeaseID clientv3.LeaseID
	cancel  context.CancelFunc
}

const (
	etcdPrefix = "/globular/services/"
	cfgKey     = "config"
	rtKey      = "runtime"
	liveKey    = "live"
)

func etcdKey(id, leaf string) string {
	return etcdPrefix + id + "/" + leaf
}

// etcdZapLoggerFromEnv returns a zap.Logger for the etcd client.
// GLOB_ETCD_LOG: silent|error|warn|info|debug (default: silent)
func etcdZapLoggerFromEnv() *zap.Logger {
	level := strings.ToLower(strings.TrimSpace(os.Getenv("GLOB_ETCD_LOG")))
	switch level {
	case "", "silent", "off", "none":
		return zap.NewNop()
	case "error":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "warn", "warning":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "info":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "debug":
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		l, _ := cfg.Build()
		return l
	default:
		return zap.NewNop()
	}
}

func etcdClient() (*clientv3.Client, error) {
	cliOnce.Do(func() {
		endpoints := detectEtcdEndpoints()
		if len(endpoints) == 0 {
			endpoints = []string{"127.0.0.1:2379"}
		}
		cli, cliErr = clientv3.New(clientv3.Config{
			Endpoints:            endpoints,
			DialTimeout:          3 * time.Second,
			DialKeepAliveTime:    10 * time.Second,
			DialKeepAliveTimeout: 3 * time.Second,
			DialOptions:          []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
			// KEY: control etcd client verbosity here
			Logger: etcdZapLoggerFromEnv(),
			// (Alternative is LogConfig, but Logger is simplest & works across etcd 3.5.x)
		})
	})
	return cli, cliErr
}

// Derive endpoints from local env (best-effort, no config lookups).
func detectEtcdEndpoints() []string {
	if v := os.Getenv("GLOBULAR_ETCD_ENDPOINTS"); v != "" {
		parts := strings.Split(v, ",")
		var eps []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				eps = append(eps, p)
			}
		}
		if len(eps) > 0 {
			return eps
		}
	}
	// safe default without touching config
	return []string{"127.0.0.1:2379"}
}

// =============================
// Desired/runtime split helpers
// =============================

var runtimeKeys = map[string]struct{}{
	"Process":      {},
	"ProxyProcess": {},
	"State":        {},
	"LastError":    {},
	"ModTime":      {}, // not desired
}

func splitDesiredRuntime(s map[string]interface{}) (desired, runtime map[string]interface{}) {
	desired = make(map[string]interface{}, len(s))
	runtime = map[string]interface{}{
		"UpdatedAt": time.Now().Unix(),
	}
	for k, v := range s {
		if _, ok := runtimeKeys[k]; ok {
			if k != "ModTime" { // ignore modtime entirely
				runtime[k] = v
			}
			continue
		}
		desired[k] = v
	}
	// Ensure ID in desired
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
	// Fill defaults if absent.
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

// =============================
// Public API (etcd-backed)
// =============================

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
	_, err = c.Put(ctx, etcdKey(id, cfgKey), string(desBytes))
	if err != nil {
		return fmt.Errorf("save desired: %w", err)
	}

	rtBytes, _ := json.Marshal(runtime)
	_, err = c.Put(ctx, etcdKey(id, rtKey), string(rtBytes))
	if err != nil {
		return fmt.Errorf("save runtime: %w", err)
	}
	return nil
}

// PutRuntime is a convenience when only runtime changes.
func PutRuntime(id string, runtime map[string]interface{}) error {
	c, err := etcdClient()
	if err != nil {
		return err
	}
	runtime["UpdatedAt"] = time.Now().Unix()
	rtBytes, _ := json.Marshal(runtime)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = c.Put(ctx, etcdKey(id, rtKey), string(rtBytes))
	return err
}

func GetServicesConfigurations() ([]map[string]interface{}, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all desired configs
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
		case cfgKey:
			var d map[string]interface{}
			if err := json.Unmarshal(kv.Value, &d); err != nil {
				continue
			}
			desiredByID[id] = d

		case rtKey:
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

func GetServiceConfigurationById(idOrName string) (map[string]interface{}, error) {
	// Try by Id exact match
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Read desired
	dres, err := c.Get(ctx, etcdKey(idOrName, cfgKey))
	if err == nil && len(dres.Kvs) == 1 {
		var d map[string]interface{}
		if json.Unmarshal(dres.Kvs[0].Value, &d) == nil {
			// runtime (optional)
			rres, _ := c.Get(ctx, etcdKey(idOrName, rtKey))
			var r map[string]interface{}
			if len(rres.Kvs) == 1 {
				_ = json.Unmarshal(rres.Kvs[0].Value, &r)
			}
			return mergeDesiredRuntime(d, r), nil
		}
	}

	// Fallback: scan all and match by Name
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if Utility.ToString(s["Id"]) == idOrName || Utility.ToString(s["Name"]) == idOrName {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no service found with id/name %q", idOrName)
}

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

// =============================
// Live lease helpers
// =============================

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

	_, err = c.Put(context.Background(), etcdKey(id, liveKey), "", clientv3.WithLease(g.ID))
	if err != nil {
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
	// drain keep-alives
	go func() { for range ch {} }()

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
	// revoke lease (deletes the /live key)
	if c, err := etcdClient(); err == nil {
		_, _ = c.Lease.Revoke(context.Background(), ll.LeaseID)
	}
}
