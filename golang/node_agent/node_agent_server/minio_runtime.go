package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
)

// minioObserverSubTimeout bounds each health request so the observer can never
// become a new availability risk.
const minioObserverSubTimeout = 1200 * time.Millisecond

// observeMinioRuntime is the production MinioRuntimeObserver for the infra truth
// plane. It reads MinIO's unauthenticated native health endpoints over HTTPS — no
// root credentials are handled here. It dials the local node's own health base
// URL (config.MinIOTLSConfig recognises the co-located node IP and keeps the
// connection local), so the observed truth is THIS node's MinIO. Every failure is
// recorded as evidence on the returned state; the probe never aborts.
func observeMinioRuntime(ctx context.Context, healthBaseURL string) *infra_truth.MinioRuntimeState {
	rt := &infra_truth.MinioRuntimeState{}

	tlsCfg, err := config.MinIOTLSConfig(healthBaseURL)
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("minio TLS unavailable: %v", err))
		return rt
	}
	client := &http.Client{
		Timeout:   minioObserverSubTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}

	// Liveness — proves the local server answers at all.
	rt.Live = minioHealthOK(ctx, client, healthBaseURL+"/minio/health/live", "live", rt)
	// Cluster write quorum — 200 means the pool can accept writes; 503 means it
	// cannot (the most important blast-radius signal). The non-200 status is
	// recorded as evidence, not absorbed.
	rt.WriteQuorum = minioHealthOK(ctx, client, healthBaseURL+"/minio/health/cluster", "cluster(write-quorum)", rt)
	// Cluster read quorum.
	rt.ReadQuorum = minioHealthOK(ctx, client, healthBaseURL+"/minio/health/cluster/read", "cluster/read(read-quorum)", rt)

	return rt
}

// minioHealthOK issues a bounded GET and returns true on HTTP 200. A transport
// error or a non-200 status is appended to rt.Errors with its class so a "false"
// is never silent (honors degraded_is_explicit_not_hidden).
func minioHealthOK(ctx context.Context, client *http.Client, url, label string, rt *infra_truth.MinioRuntimeState) bool {
	reqCtx, cancel := context.WithTimeout(ctx, minioObserverSubTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("minio health %s: build request: %v", label, err))
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("minio health %s: %v", label, err))
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		rt.Errors = append(rt.Errors, fmt.Sprintf("minio health %s: status %d", label, resp.StatusCode))
		return false
	}
	return true
}
