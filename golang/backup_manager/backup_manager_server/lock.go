package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	clusterLockKey = "/globular/backup/locks/cluster"
	lockLeaseTTL   = 600 // 10 minutes — long enough for a full cluster backup
)

// ClusterLock holds the etcd mutex and session for a cluster-wide backup lock.
//go:schemalint:ignore — implementation type, not schema owner
type ClusterLock struct {
	session *concurrency.Session
	mutex   *concurrency.Mutex
	client  *clientv3.Client
}

// AcquireClusterLock obtains an etcd-backed distributed lock for cluster backups.
// Returns a ClusterLock that must be released via Release().
// Returns an error immediately if the lock is already held.
func (srv *server) AcquireClusterLock(ctx context.Context, jobID, backupID string) (*ClusterLock, error) {
	cli, err := srv.etcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}

	session, err := concurrency.NewSession(cli, concurrency.WithTTL(lockLeaseTTL))
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("create etcd session: %w", err)
	}

	mutex := concurrency.NewMutex(session, clusterLockKey)

	// Try to acquire with a short timeout — don't block waiting
	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := mutex.TryLock(lockCtx); err != nil {
		session.Close()
		cli.Close()
		return nil, fmt.Errorf("cluster backup already in progress (lock held at %s)", clusterLockKey)
	}

	slog.Info("cluster lock acquired", "job_id", jobID, "backup_id", backupID, "key", clusterLockKey)

	return &ClusterLock{
		session: session,
		mutex:   mutex,
		client:  cli,
	}, nil
}

// Release releases the distributed lock and closes the etcd session.
func (cl *ClusterLock) Release() {
	if cl == nil {
		return
	}
	if cl.mutex != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cl.mutex.Unlock(ctx); err != nil {
			slog.Warn("failed to unlock cluster lock", "error", err)
		} else {
			slog.Info("cluster lock released")
		}
	}
	if cl.session != nil {
		cl.session.Close()
	}
	if cl.client != nil {
		cl.client.Close()
	}
}

// etcdClient creates a new etcd client using the backup-manager's configuration.
func (srv *server) etcdClient() (*clientv3.Client, error) {
	endpoints := strings.Split(srv.EtcdEndpoints, ",")
	for i, ep := range endpoints {
		endpoints[i] = strings.TrimSpace(ep)
	}

	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}

	// Configure TLS if certs are available
	if fileExists(srv.EtcdCACert) && fileExists(srv.EtcdCert) && fileExists(srv.EtcdKey) {
		cert, err := tls.LoadX509KeyPair(srv.EtcdCert, srv.EtcdKey)
		if err != nil {
			return nil, fmt.Errorf("load etcd client cert: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}
		// Load CA if available
		caData, err := os.ReadFile(srv.EtcdCACert)
		if err == nil && len(caData) > 0 {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caData)
			tlsCfg.RootCAs = pool
		}
		cfg.TLS = tlsCfg
	}

	return clientv3.New(cfg)
}
