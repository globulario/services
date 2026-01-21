package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type nodeAgentState struct {
	ControllerEndpoint string `json:"controller_endpoint"`
	RequestID          string `json:"request_id"`
	NodeID             string `json:"node_id"`
	LastPlanGeneration uint64 `json:"last_plan_generation"`
	NetworkGeneration  uint64 `json:"network_generation"`
	ClusterDomain      string `json:"cluster_domain"`
	Protocol           string `json:"protocol"`
	CertGeneration     uint64 `json:"cert_generation"`
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
