package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type nodeAgentState struct {
	ControllerEndpoint string `json:"controller_endpoint"`
	ControllerInsecure bool   `json:"controller_insecure"`
	RequestID          string `json:"request_id"`
	NodeID             string `json:"node_id"`
	JoinToken          string `json:"join_token"`
	NetworkGeneration  uint64 `json:"network_generation"`
	ClusterDomain      string `json:"cluster_domain"`
	Protocol           string `json:"protocol"`
	CertGeneration     uint64 `json:"cert_generation"`

	// DNS-first naming fields (PR1)
	NodeName      string `json:"node_name"`
	AdvertiseIP   string `json:"advertise_ip"`
	AdvertiseFQDN string `json:"advertise_fqdn"`

	// v2 join path: join_id is set by the installer via /join/authorize before
	// any cluster-affecting step. The node-agent uses it as the request_id for
	// GetJoinRequestStatus polling, bypassing the v1 RequestJoin call.
	JoinID string `json:"join_id,omitempty"`
	// JoinPlanJSON is the raw JoinPlan JSON written by the installer alongside
	// the join_id. The node-agent validates the plan before polling for admission.
	JoinPlanJSON []byte `json:"join_plan_json,omitempty"`
}

func newNodeAgentState() *nodeAgentState {
	return &nodeAgentState{}
}

func loadNodeAgentState(path string) (*nodeAgentState, error) {
	state := newNodeAgentState()
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
	return state, nil
}

func (s *nodeAgentState) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "nodeagent-state-*.tmp")
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
