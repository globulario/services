package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPrometheusFiltersZeroNodeHeartbeatTimestamp(t *testing.T) {
	var heartbeatQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		if strings.Contains(q, "globular_node_agent_heartbeat_success_unix") {
			heartbeatQuery = q
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []map[string]any{{
					"value": []any{1234.0, "1"},
				}},
			},
		})
	}))
	defer srv.Close()

	c := &Collector{promEndpoint: srv.URL}
	c.fetchPrometheus(context.Background(), &Snapshot{})

	if heartbeatQuery == "" {
		t.Fatal("expected node heartbeat query")
	}
	if !strings.Contains(heartbeatQuery, "globular_node_agent_heartbeat_success_unix > 0") {
		t.Fatalf("node heartbeat query must filter zero/default timestamps; got %q", heartbeatQuery)
	}
}
