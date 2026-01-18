package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNodeHasProfile(t *testing.T) {
	tests := []struct {
		name     string
		node     *memberNode
		profiles []string
		want     bool
	}{
		{
			name:     "nil node",
			node:     nil,
			profiles: []string{"core"},
			want:     false,
		},
		{
			name: "node with matching profile",
			node: &memberNode{
				NodeID:   "n1",
				Profiles: []string{"core", "gateway"},
			},
			profiles: []string{"core"},
			want:     true,
		},
		{
			name: "node without matching profile",
			node: &memberNode{
				NodeID:   "n1",
				Profiles: []string{"gateway"},
			},
			profiles: []string{"core", "storage"},
			want:     false,
		},
		{
			name: "case insensitive match",
			node: &memberNode{
				NodeID:   "n1",
				Profiles: []string{"Core"},
			},
			profiles: []string{"CORE"},
			want:     true,
		},
		{
			name: "empty profiles on node",
			node: &memberNode{
				NodeID:   "n1",
				Profiles: []string{},
			},
			profiles: []string{"core"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeHasProfile(tt.node, tt.profiles)
			if got != tt.want {
				t.Errorf("nodeHasProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterNodesByProfile(t *testing.T) {
	membership := &clusterMembership{
		ClusterID: "test-cluster",
		Nodes: []memberNode{
			{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
			{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"gateway"}},
			{NodeID: "n3", Hostname: "host3", IP: "192.168.1.12", Profiles: []string{"core", "storage"}},
			{NodeID: "n4", Hostname: "host4", IP: "", Profiles: []string{"core"}}, // no IP, should be filtered out
		},
	}

	tests := []struct {
		name       string
		profiles   []string
		wantCount  int
		wantNodeID string
	}{
		{
			name:       "filter by core profile",
			profiles:   []string{"core"},
			wantCount:  2, // n1 and n3 (n4 has no IP)
			wantNodeID: "n1",
		},
		{
			name:       "filter by gateway profile",
			profiles:   []string{"gateway"},
			wantCount:  1,
			wantNodeID: "n2",
		},
		{
			name:       "filter by multiple profiles",
			profiles:   []string{"core", "gateway"},
			wantCount:  3, // n1, n2, n3
			wantNodeID: "n1",
		},
		{
			name:      "filter by non-existent profile",
			profiles:  []string{"compute"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterNodesByProfile(membership, tt.profiles)
			if len(result) != tt.wantCount {
				t.Errorf("filterNodesByProfile() count = %v, want %v", len(result), tt.wantCount)
			}
			if tt.wantCount > 0 && result[0].NodeID != tt.wantNodeID {
				t.Errorf("filterNodesByProfile() first node = %v, want %v", result[0].NodeID, tt.wantNodeID)
			}
		})
	}
}

func TestSanitizeEtcdName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"host1", "host1"},
		{"host.example.com", "host-example-com"},
		{"my_host", "my_host"},
		{"host-1", "host-1"},
		{"", "node"},
		{"...", "node"},
		{"---", "node"},
		{"host@domain", "host-domain"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeEtcdName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeEtcdName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderEtcdConfig(t *testing.T) {
	t.Run("single node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
			ClusterID:   "test-cluster",
		}

		config, ok := renderEtcdConfig(ctx)
		if !ok {
			t.Fatal("renderEtcdConfig() returned false")
		}

		// Check for expected keys
		if !strings.Contains(config, `name: "host1"`) {
			t.Error("config missing name field")
		}
		if !strings.Contains(config, `data-dir: "/var/lib/globular/etcd"`) {
			t.Error("config missing data-dir field")
		}
		if !strings.Contains(config, "initial-cluster-token") {
			t.Error("config missing initial-cluster-token field")
		}
		// Single node should use localhost only
		if !strings.Contains(config, `listen-client-urls: "http://127.0.0.1:2379"`) {
			t.Error("single node should use localhost for listen-client-urls")
		}
	})

	t.Run("multi-node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
					{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
			ClusterID:   "test-cluster",
		}

		config, ok := renderEtcdConfig(ctx)
		if !ok {
			t.Fatal("renderEtcdConfig() returned false")
		}

		// Check for initial-cluster with both nodes
		if !strings.Contains(config, "192.168.1.10:2380") {
			t.Error("config missing first node peer URL")
		}
		if !strings.Contains(config, "192.168.1.11:2380") {
			t.Error("config missing second node peer URL")
		}
		// Multi-node should have both localhost and IP in listen-client-urls
		if !strings.Contains(config, "192.168.1.10:2379") && !strings.Contains(config, "127.0.0.1:2379") {
			t.Error("multi-node should have IP in listen-client-urls")
		}
	})

	t.Run("node without etcd profile", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"gateway"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"gateway"}},
			ClusterID:   "test-cluster",
		}

		_, ok := renderEtcdConfig(ctx)
		if ok {
			t.Error("renderEtcdConfig() should return false for node without etcd profile")
		}
	})
}

func TestRenderMinioConfig(t *testing.T) {
	t.Run("single node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
		}

		config, ok := renderMinioConfig(ctx)
		if !ok {
			t.Fatal("renderMinioConfig() returned false")
		}

		if !strings.Contains(config, "MINIO_VOLUMES=/var/lib/globular/minio/data") {
			t.Error("single node should use local path")
		}
		if strings.Contains(config, "http://") {
			t.Error("single node should not have HTTP endpoints")
		}
	})

	t.Run("multi-node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
					{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"storage"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
		}

		config, ok := renderMinioConfig(ctx)
		if !ok {
			t.Fatal("renderMinioConfig() returned false")
		}

		if !strings.Contains(config, "http://192.168.1.10:9000") {
			t.Error("config missing first node endpoint")
		}
		if !strings.Contains(config, "http://192.168.1.11:9000") {
			t.Error("config missing second node endpoint")
		}
	})

	t.Run("node without minio profile", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"gateway"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"gateway"}},
		}

		_, ok := renderMinioConfig(ctx)
		if ok {
			t.Error("renderMinioConfig() should return false for node without minio profile")
		}
	})
}

func TestRenderXDSConfig(t *testing.T) {
	t.Run("single node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
			Domain:      "example.com",
		}

		config, ok := renderXDSConfig(ctx)
		if !ok {
			t.Fatal("renderXDSConfig() returned false")
		}

		// Parse the JSON and verify structure
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		endpoints, ok := parsed["etcd_endpoints"].([]interface{})
		if !ok {
			t.Fatal("missing etcd_endpoints")
		}
		if len(endpoints) != 1 {
			t.Errorf("expected 1 endpoint, got %d", len(endpoints))
		}
		if endpoints[0] != "192.168.1.10:2379" {
			t.Errorf("unexpected endpoint: %v", endpoints[0])
		}

		if parsed["sync_interval_seconds"] != float64(5) {
			t.Error("missing or incorrect sync_interval_seconds")
		}

		// Check TLS paths include the domain
		ingress, ok := parsed["ingress"].(map[string]interface{})
		if !ok {
			t.Fatal("missing ingress config")
		}
		tls, ok := ingress["tls"].(map[string]interface{})
		if !ok {
			t.Fatal("missing tls config")
		}
		certPath, _ := tls["cert_chain_path"].(string)
		if !strings.Contains(certPath, "example.com") {
			t.Errorf("cert path should include domain: %s", certPath)
		}
	})

	t.Run("multi-node cluster with multiple etcd nodes", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
					{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"control-plane"}},
					{NodeID: "n3", Hostname: "host3", IP: "192.168.1.12", Profiles: []string{"gateway"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n3", Hostname: "host3", IP: "192.168.1.12", Profiles: []string{"gateway"}},
			Domain:      "test.example.com",
		}

		config, ok := renderXDSConfig(ctx)
		if !ok {
			t.Fatal("renderXDSConfig() returned false")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		endpoints, ok := parsed["etcd_endpoints"].([]interface{})
		if !ok {
			t.Fatal("missing etcd_endpoints")
		}
		// Should have 2 endpoints (n1 with core, n2 with control-plane)
		if len(endpoints) != 2 {
			t.Errorf("expected 2 endpoints, got %d", len(endpoints))
		}
	})

	t.Run("node without xds profile", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
		}

		_, ok := renderXDSConfig(ctx)
		if ok {
			t.Error("renderXDSConfig() should return false for node without xds profile")
		}
	})
}

func TestRenderDNSConfig(t *testing.T) {
	t.Run("single node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "ns1", IP: "192.168.1.10", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "ns1", IP: "192.168.1.10", Profiles: []string{"core"}},
			Domain:      "example.com",
		}

		config, ok := renderDNSConfig(ctx)
		if !ok {
			t.Fatal("renderDNSConfig() returned false")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		// Check domain
		if parsed["domain"] != "example.com" {
			t.Errorf("unexpected domain: %v", parsed["domain"])
		}

		// Check is_primary (first node should be primary)
		if parsed["is_primary"] != true {
			t.Error("first node should be primary")
		}

		// Check SOA record
		soa, ok := parsed["soa"].(map[string]interface{})
		if !ok {
			t.Fatal("missing soa record")
		}
		if soa["domain"] != "example.com" {
			t.Errorf("unexpected SOA domain: %v", soa["domain"])
		}
		if !strings.HasPrefix(soa["ns"].(string), "ns1.example.com") {
			t.Errorf("unexpected SOA ns: %v", soa["ns"])
		}

		// Check NS records
		nsRecords, ok := parsed["ns_records"].([]interface{})
		if !ok || len(nsRecords) != 1 {
			t.Fatalf("expected 1 NS record, got %v", nsRecords)
		}

		// Check glue records
		glueRecords, ok := parsed["glue_records"].([]interface{})
		if !ok || len(glueRecords) != 1 {
			t.Fatalf("expected 1 glue record, got %v", glueRecords)
		}
		glue := glueRecords[0].(map[string]interface{})
		if glue["ip"] != "192.168.1.10" {
			t.Errorf("unexpected glue IP: %v", glue["ip"])
		}
	})

	t.Run("multi-node cluster", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "ns1", IP: "192.168.1.10", Profiles: []string{"dns"}},
					{NodeID: "n2", Hostname: "ns2", IP: "192.168.1.11", Profiles: []string{"dns"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n2", Hostname: "ns2", IP: "192.168.1.11", Profiles: []string{"dns"}},
			Domain:      "example.com",
		}

		config, ok := renderDNSConfig(ctx)
		if !ok {
			t.Fatal("renderDNSConfig() returned false")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		// n2 should not be primary (n1 is first alphabetically)
		if parsed["is_primary"] != false {
			t.Error("second node should not be primary")
		}

		// Should have 2 NS records
		nsRecords, ok := parsed["ns_records"].([]interface{})
		if !ok || len(nsRecords) != 2 {
			t.Fatalf("expected 2 NS records, got %v", nsRecords)
		}

		// Should have 2 glue records
		glueRecords, ok := parsed["glue_records"].([]interface{})
		if !ok || len(glueRecords) != 2 {
			t.Fatalf("expected 2 glue records, got %v", glueRecords)
		}
	})

	t.Run("node without dns profile", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"storage"}},
			Domain:      "example.com",
		}

		_, ok := renderDNSConfig(ctx)
		if ok {
			t.Error("renderDNSConfig() should return false for node without dns profile")
		}
	})
}

func TestGenerateSOASerial(t *testing.T) {
	serial := generateSOASerial()
	// Serial should be at least YYYYMMDD00 format (2024010100 minimum)
	if serial < 2024010100 {
		t.Errorf("serial seems too low: %d", serial)
	}
	// Serial should not exceed reasonable bounds (2099123199)
	if serial > 2099123199 {
		t.Errorf("serial seems too high: %d", serial)
	}
}

func TestRenderServiceConfigs(t *testing.T) {
	t.Run("core profile gets all configs", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
			ClusterID:   "test-cluster",
			Domain:      "example.com",
		}

		configs := renderServiceConfigs(ctx)
		if configs == nil {
			t.Fatal("renderServiceConfigs() returned nil")
		}

		expectedPaths := []string{
			"/var/lib/globular/etcd/etcd.yaml",
			"/var/lib/globular/minio/minio.env",
			"/var/lib/globular/xds/config.json",
			"/var/lib/globular/dns/dns_init.json",
		}

		for _, path := range expectedPaths {
			if _, ok := configs[path]; !ok {
				t.Errorf("missing config for path: %s", path)
			}
		}
	})

	t.Run("gateway profile gets only xds config", func(t *testing.T) {
		ctx := &serviceConfigContext{
			Membership: &clusterMembership{
				ClusterID: "test-cluster",
				Nodes: []memberNode{
					{NodeID: "n1", Hostname: "host1", IP: "192.168.1.10", Profiles: []string{"core"}},
					{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"gateway"}},
				},
			},
			CurrentNode: &memberNode{NodeID: "n2", Hostname: "host2", IP: "192.168.1.11", Profiles: []string{"gateway"}},
			ClusterID:   "test-cluster",
			Domain:      "example.com",
		}

		configs := renderServiceConfigs(ctx)
		if configs == nil {
			t.Fatal("renderServiceConfigs() returned nil")
		}

		if _, ok := configs["/var/lib/globular/xds/config.json"]; !ok {
			t.Error("gateway should have xds config")
		}
		if _, ok := configs["/var/lib/globular/etcd/etcd.yaml"]; ok {
			t.Error("gateway should not have etcd config")
		}
		if _, ok := configs["/var/lib/globular/minio/minio.env"]; ok {
			t.Error("gateway should not have minio config")
		}
	})

	t.Run("nil context returns nil", func(t *testing.T) {
		configs := renderServiceConfigs(nil)
		if configs != nil {
			t.Error("nil context should return nil")
		}
	})
}

func TestRenderYAML(t *testing.T) {
	data := map[string]interface{}{
		"name":    "test",
		"port":    8080,
		"enabled": true,
	}

	yaml, err := renderYAML(data)
	if err != nil {
		t.Fatalf("renderYAML() error: %v", err)
	}

	if !strings.Contains(yaml, `name: "test"`) {
		t.Error("missing name field")
	}
	if !strings.Contains(yaml, "port: 8080") {
		t.Error("missing port field")
	}
	if !strings.Contains(yaml, "enabled: true") {
		t.Error("missing enabled field")
	}
}

func TestRenderJSON(t *testing.T) {
	data := map[string]interface{}{
		"name":    "test",
		"port":    8080,
		"enabled": true,
	}

	jsonStr, err := renderJSON(data)
	if err != nil {
		t.Fatalf("renderJSON() error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["name"] != "test" {
		t.Error("name field mismatch")
	}
	if parsed["port"] != float64(8080) {
		t.Error("port field mismatch")
	}
	if parsed["enabled"] != true {
		t.Error("enabled field mismatch")
	}
}
