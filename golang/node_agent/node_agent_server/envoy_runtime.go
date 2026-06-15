package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
)

// envoyObserverSubTimeout bounds each admin request so the observer can never
// become a new availability risk.
const envoyObserverSubTimeout = 1200 * time.Millisecond

// observeEnvoyRuntime is the production EnvoyRuntimeObserver. It reads Envoy's
// native admin API (/ready, /server_info, /stats) over plain HTTP — the admin
// interface is loopback by design, so no TLS. Every failure is recorded as
// evidence on the returned state; the probe never aborts.
func observeEnvoyRuntime(ctx context.Context, adminBaseURL string) *infra_truth.EnvoyRuntimeState {
	rt := &infra_truth.EnvoyRuntimeState{}
	client := &http.Client{Timeout: envoyObserverSubTimeout}

	// /ready — 200 means fully initialized. Any HTTP response proves the admin
	// interface is reachable (even a 503 from a still-initializing server).
	if status, _, err := envoyAdminGet(ctx, client, adminBaseURL+"/ready"); err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("envoy /ready: %v", err))
	} else {
		rt.AdminReachable = true
		rt.Ready = status == http.StatusOK
	}

	// /server_info — state + version.
	if status, body, err := envoyAdminGet(ctx, client, adminBaseURL+"/server_info"); err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("envoy /server_info: %v", err))
	} else if status == http.StatusOK {
		rt.AdminReachable = true
		var info struct {
			Version string `json:"version"`
			State   string `json:"state"`
		}
		if err := json.Unmarshal(body, &info); err == nil {
			rt.ServerState = info.State
			rt.Version = info.Version
		}
	}

	// /stats?format=json — the xDS handshake counters.
	if status, body, err := envoyAdminGet(ctx, client, adminBaseURL+"/stats?format=json"); err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("envoy /stats: %v", err))
	} else if status == http.StatusOK {
		rt.AdminReachable = true
		applyEnvoyStats(body, rt)
	}

	return rt
}

// envoyAdminGet issues a bounded GET and returns the status code and body.
func envoyAdminGet(ctx context.Context, client *http.Client, url string) (int, []byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, envoyObserverSubTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// applyEnvoyStats parses the /stats?format=json payload and extracts the counters
// the truth plane cares about. The payload shape is
// {"stats":[{"name":"...","value":N}, {"name":"...","histograms":{...}}, ...]}.
func applyEnvoyStats(body []byte, rt *infra_truth.EnvoyRuntimeState) {
	var doc struct {
		Stats []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("envoy /stats decode: %v", err))
		return
	}
	for _, s := range doc.Stats {
		if len(s.Value) == 0 {
			continue // histogram entry, not a scalar
		}
		var n int64
		if err := json.Unmarshal(s.Value, &n); err != nil {
			continue
		}
		switch s.Name {
		case "cluster_manager.cds.update_success":
			rt.CDSUpdateSuccess = n
		case "cluster_manager.active_clusters":
			rt.ActiveClusters = n
		case "listener_manager.lds.update_attempt":
			rt.LDSUpdateAttempt = n
		case "listener_manager.lds.update_success":
			rt.LDSUpdateSuccess = n
		case "listener_manager.lds.update_rejected":
			rt.LDSUpdateRejected = n
		case "listener_manager.total_listeners_active":
			rt.ActiveListeners = n
		}
	}
}
