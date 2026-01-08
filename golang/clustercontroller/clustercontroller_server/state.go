package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/google/uuid"
)

const defaultClusterStatePath = "/var/lib/globular/clustercontroller/state.json"

type controllerState struct {
	JoinTokens           map[string]*joinTokenRecord             `json:"join_tokens"`
	JoinRequests         map[string]*joinRequestRecord           `json:"join_requests"`
	Nodes                map[string]*nodeState                   `json:"nodes"`
	ClusterId            string                                  `json:"cluster_id"`
	CreatedAt            time.Time                               `json:"created_at"`
	ClusterNetworkSpec   *clustercontrollerpb.ClusterNetworkSpec `json:"cluster_network_spec,omitempty"`
	NetworkingGeneration uint64                                  `json:"networking_generation"`
}

type joinTokenRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   int       `json:"max_uses"`
	Uses      int       `json:"uses"`
}

type joinRequestRecord struct {
	RequestID      string            `json:"request_id"`
	Token          string            `json:"token"`
	Identity       storedIdentity    `json:"identity"`
	Labels         map[string]string `json:"labels"`
	RequestedAt    time.Time         `json:"requested_at"`
	Status         string            `json:"status"`
	Reason         string            `json:"reason,omitempty"`
	Profiles       []string          `json:"profiles,omitempty"`
	AssignedNodeID string            `json:"assigned_node_id,omitempty"`
}

type nodeState struct {
	NodeID                string             `json:"node_id"`
	Identity              storedIdentity     `json:"identity"`
	Profiles              []string           `json:"profiles"`
	LastSeen              time.Time          `json:"last_seen"`
	Status                string             `json:"status"`
	Metadata              map[string]string  `json:"metadata,omitempty"`
	AgentEndpoint         string             `json:"agent_endpoint,omitempty"`
	Units                 []unitStatusRecord `json:"units,omitempty"`
	LastError             string             `json:"last_error,omitempty"`
	ReportedAt            time.Time          `json:"reported_at,omitempty"`
	LastPlanSentAt        time.Time          `json:"last_plan_sent_at,omitempty"`
	LastPlanError         string             `json:"last_plan_error,omitempty"`
	LastPlanHash          string             `json:"last_plan_hash,omitempty"`
	LastAppliedGeneration uint64             `json:"last_applied_generation,omitempty"`
}

type unitStatusRecord struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Details string `json:"details"`
}

type storedIdentity struct {
	Hostname     string   `json:"hostname"`
	Domain       string   `json:"domain"`
	Ips          []string `json:"ips"`
	Os           string   `json:"os"`
	Arch         string   `json:"arch"`
	AgentVersion string   `json:"agent_version"`
}

func newControllerState() *controllerState {
	return &controllerState{
		JoinTokens:   make(map[string]*joinTokenRecord),
		JoinRequests: make(map[string]*joinRequestRecord),
		Nodes:        make(map[string]*nodeState),
		ClusterId:    uuid.NewString(),
		CreatedAt:    time.Now(),
	}
}

func loadControllerState(path string) (*controllerState, error) {
	state := newControllerState()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(b, state); err != nil {
		return nil, err
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now()
	}
	if state.ClusterId == "" {
		state.ClusterId = uuid.NewString()
	}
	return state, nil
}

func (s *controllerState) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
