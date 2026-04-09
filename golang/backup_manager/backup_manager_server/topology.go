package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TopologySnapshot captures the cluster state at backup time.
type TopologySnapshot struct {
	CapturedAt string            `json:"captured_at"`
	Domain     string            `json:"domain"`
	Nodes      []TopologyNode    `json:"nodes"`
	Services   []TopologyService `json:"services"`
	Endpoints  TopologyEndpoints `json:"endpoints"`
}

// TopologyNode describes a single cluster node.
type TopologyNode struct {
	NodeID        string            `json:"node_id"`
	Hostname      string            `json:"hostname"`
	Address       string            `json:"address"`
	AgentEndpoint string            `json:"agent_endpoint"`
	Status        string            `json:"status"`
	Profiles      []string          `json:"profiles,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// TopologyService describes a service instance in the cluster.
type TopologyService struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Version string `json:"version"`
	State   string `json:"state"`
}

// TopologyEndpoints captures the infrastructure endpoints at backup time.
type TopologyEndpoints struct {
	Etcd   string `json:"etcd"`
	Scylla string `json:"scylla,omitempty"`
	Minio  string `json:"minio,omitempty"`
}

// captureTopology collects cluster topology and writes it into the backup capsule.
func (srv *server) captureTopology(backupID string) (*backup_managerpb.ClusterInfo, error) {
	snapshot := TopologySnapshot{
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Domain:     srv.Domain,
		Endpoints: TopologyEndpoints{
			Etcd:   srv.EtcdEndpoints,
			Scylla: srv.ScyllaManagerAPI,
			Minio:  srv.RcloneRemote,
		},
	}

	snapshot.Nodes = srv.discoverNodes()
	snapshot.Services = discoverServices()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal topology: %w", err)
	}

	capsuleDir := srv.CapsuleDir(backupID)
	if err := CapsuleWriteFile(capsuleDir, "meta/topology.json", data); err != nil {
		return nil, fmt.Errorf("write topology.json: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	_ = CapsuleWriteFile(capsuleDir, "meta/topology.sha256", []byte(hash+"\n"))

	slog.Info("topology snapshot captured",
		"backup_id", backupID,
		"nodes", len(snapshot.Nodes),
		"services", len(snapshot.Services),
		"hash", hash[:16])

	return &backup_managerpb.ClusterInfo{
		ClusterId:    srv.Domain,
		Domain:       srv.Domain,
		TopologyHash: hash,
	}, nil
}

// discoverNodes queries the cluster controller for the current node list.
func (srv *server) discoverNodes() []TopologyNode {
	nodes, err := srv.listNodesFromController()
	if err != nil {
		slog.Warn("could not discover cluster nodes, using local node", "error", err)
		hostname, _ := os.Hostname()
		return []TopologyNode{{
			NodeID:   srv.Id,
			Hostname: hostname,
			Address:  srv.Address,
			Status:   "self",
		}}
	}
	return nodes
}

// listNodesFromController calls the cluster controller's ListNodes RPC.
func (srv *server) listNodesFromController() ([]TopologyNode, error) {
	addr := config.ResolveServiceAddr(
		"cluster_controller.ClusterControllerService",
		"",
	)
	if addr == "" {
		return nil, fmt.Errorf("cluster controller address not found in etcd")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cluster controller runs plain gRPC (no TLS)
	cc, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial cluster controller at %s: %w", addr, err)
	}
	defer cc.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	resp, err := client.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListNodes RPC: %w", err)
	}

	var nodes []TopologyNode
	for _, n := range resp.Nodes {
		tn := TopologyNode{
			NodeID:        n.NodeId,
			Status:        n.Status,
			AgentEndpoint: n.AgentEndpoint,
			Profiles:      n.Profiles,
			Metadata:      n.Metadata,
		}
		if n.Identity != nil {
			tn.Hostname = n.Identity.Hostname
			tn.Address = n.Identity.AdvertiseIp
			if tn.Address == "" && len(n.Identity.Ips) > 0 {
				tn.Address = n.Identity.Ips[0]
			}
		}
		nodes = append(nodes, tn)
	}
	return nodes, nil
}

// discoverServices lists all services from the etcd registry.
func discoverServices() []TopologyService {
	cfgs, err := config.GetServicesConfigurations()
	if err != nil {
		slog.Warn("could not discover services from etcd", "error", err)
		return nil
	}

	var services []TopologyService
	for _, c := range cfgs {
		name, _ := c["Name"].(string)
		id, _ := c["Id"].(string)
		domain, _ := c["Domain"].(string)
		address, _ := c["Address"].(string)
		version, _ := c["Version"].(string)
		state, _ := c["State"].(string)

		var port int
		switch p := c["Port"].(type) {
		case float64:
			port = int(p)
		case int:
			port = p
		}

		services = append(services, TopologyService{
			ID:      id,
			Name:    name,
			Domain:  domain,
			Address: address,
			Port:    port,
			Version: version,
			State:   state,
		})
	}
	return services
}
