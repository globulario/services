package keepalived

import (
	"fmt"
	"strings"
	"testing"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ingress"
)

func TestRenderConfig_Deterministic(t *testing.T) {
	// Test that rendering the same input multiple times produces identical output
	input := RenderInput{
		NodeID:   "n1",
		Priority: 120,
		Spec: ingress.VIPFailoverSpec{
			VIP:              "10.0.0.250/24",
			Interface:        "eth0",
			VirtualRouterID:  51,
			AdvertIntervalMs: 1000,
			AuthPass:         "secret123",
			CheckTCPPorts:    []int{443, 8080},
		},
	}

	// Render 10 times
	var outputs []string
	for i := 0; i < 10; i++ {
		output, err := RenderConfig(input)
		if err != nil {
			t.Fatalf("RenderConfig failed on iteration %d: %v", i, err)
		}
		outputs = append(outputs, output)
	}

	// All outputs should be identical
	first := outputs[0]
	for i, output := range outputs {
		if output != first {
			t.Errorf("Output %d differs from first output", i)
		}
	}
}

func TestRenderConfig_DifferentPriorities(t *testing.T) {
	// Test that different nodes get different priorities in the config
	baseSpec := ingress.VIPFailoverSpec{
		VIP:              "10.0.0.250/24",
		Interface:        "eth0",
		VirtualRouterID:  51,
		AdvertIntervalMs: 1000,
		CheckTCPPorts:    []int{443},
	}

	tests := []struct {
		nodeID   string
		priority int
	}{
		{"n1", 120},
		{"n2", 110},
		{"n3", 100},
	}

	for _, tt := range tests {
		input := RenderInput{
			NodeID:   tt.nodeID,
			Priority: tt.priority,
			Spec:     baseSpec,
		}

		output, err := RenderConfig(input)
		if err != nil {
			t.Fatalf("RenderConfig failed for node %s: %v", tt.nodeID, err)
		}

		// Check that the priority appears in the config
		expectedPriority := fmt.Sprintf("priority %d", tt.priority)
		if !strings.Contains(output, expectedPriority) {
			t.Errorf("Config for node %s does not contain expected priority %d", tt.nodeID, tt.priority)
		}

		// Check that node ID appears in comment
		if !strings.Contains(output, "# Node: "+tt.nodeID) {
			t.Errorf("Config does not contain node ID %s in comment", tt.nodeID)
		}
	}
}

func TestRenderConfig_CIDRHandling(t *testing.T) {
	tests := []struct {
		name        string
		inputVIP    string
		expectedVIP string
	}{
		{
			name:        "VIP with CIDR notation",
			inputVIP:    "10.0.0.250/24",
			expectedVIP: "10.0.0.250/24",
		},
		{
			name:        "VIP without CIDR notation (IPv4)",
			inputVIP:    "10.0.0.250",
			expectedVIP: "10.0.0.250/32",
		},
		{
			name:        "VIP with /32 CIDR",
			inputVIP:    "10.0.0.250/32",
			expectedVIP: "10.0.0.250/32",
		},
		{
			name:        "IPv6 VIP without CIDR",
			inputVIP:    "2001:db8::1",
			expectedVIP: "2001:db8::1/128",
		},
		{
			name:        "IPv6 VIP with CIDR",
			inputVIP:    "2001:db8::1/64",
			expectedVIP: "2001:db8::1/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RenderInput{
				NodeID:   "n1",
				Priority: 120,
				Spec: ingress.VIPFailoverSpec{
					VIP:              tt.inputVIP,
					Interface:        "eth0",
					VirtualRouterID:  51,
					AdvertIntervalMs: 1000,
					CheckTCPPorts:    []int{443},
				},
			}

			output, err := RenderConfig(input)
			if err != nil {
				t.Fatalf("RenderConfig failed: %v", err)
			}

			// Check that the expected VIP appears in the config
			if !strings.Contains(output, tt.expectedVIP) {
				t.Errorf("Config does not contain expected VIP %s\nGot output:\n%s", tt.expectedVIP, output)
			}
		})
	}
}

func TestRenderConfig_AuthPass(t *testing.T) {
	tests := []struct {
		name     string
		authPass string
		wantAuth bool
	}{
		{
			name:     "With auth password",
			authPass: "secret123",
			wantAuth: true,
		},
		{
			name:     "Without auth password",
			authPass: "",
			wantAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RenderInput{
				NodeID:   "n1",
				Priority: 120,
				Spec: ingress.VIPFailoverSpec{
					VIP:              "10.0.0.250/24",
					Interface:        "eth0",
					VirtualRouterID:  51,
					AdvertIntervalMs: 1000,
					AuthPass:         tt.authPass,
					CheckTCPPorts:    []int{443},
				},
			}

			output, err := RenderConfig(input)
			if err != nil {
				t.Fatalf("RenderConfig failed: %v", err)
			}

			hasAuth := strings.Contains(output, "authentication {")
			if hasAuth != tt.wantAuth {
				t.Errorf("Expected authentication block: %v, but got: %v", tt.wantAuth, hasAuth)
			}

			if tt.wantAuth {
				if !strings.Contains(output, "auth_pass "+tt.authPass) {
					t.Errorf("Config does not contain auth_pass with correct password")
				}
			}
		})
	}
}

func TestRenderConfig_HealthScript(t *testing.T) {
	tests := []struct {
		name             string
		checkTCPPorts    []int
		wantHealthScript bool
	}{
		{
			name:             "With TCP health checks",
			checkTCPPorts:    []int{443, 8080},
			wantHealthScript: true,
		},
		{
			name:             "Without TCP health checks",
			checkTCPPorts:    []int{},
			wantHealthScript: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RenderInput{
				NodeID:   "n1",
				Priority: 120,
				Spec: ingress.VIPFailoverSpec{
					VIP:              "10.0.0.250/24",
					Interface:        "eth0",
					VirtualRouterID:  51,
					AdvertIntervalMs: 1000,
					CheckTCPPorts:    tt.checkTCPPorts,
				},
			}

			output, err := RenderConfig(input)
			if err != nil {
				t.Fatalf("RenderConfig failed: %v", err)
			}

			hasScript := strings.Contains(output, "vrrp_script chk_ingress")
			if hasScript != tt.wantHealthScript {
				t.Errorf("Expected health script: %v, but got: %v", tt.wantHealthScript, hasScript)
			}

			hasTrackScript := strings.Contains(output, "track_script")
			if hasTrackScript != tt.wantHealthScript {
				t.Errorf("Expected track_script: %v, but got: %v", tt.wantHealthScript, hasTrackScript)
			}
		})
	}
}

func TestRenderConfig_AdvertInterval(t *testing.T) {
	tests := []struct {
		name             string
		advertIntervalMs int
		expectedSeconds  int
	}{
		{
			name:             "1 second",
			advertIntervalMs: 1000,
			expectedSeconds:  1,
		},
		{
			name:             "2 seconds",
			advertIntervalMs: 2000,
			expectedSeconds:  2,
		},
		{
			name:             "Less than 1 second (default to 1)",
			advertIntervalMs: 500,
			expectedSeconds:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RenderInput{
				NodeID:   "n1",
				Priority: 120,
				Spec: ingress.VIPFailoverSpec{
					VIP:              "10.0.0.250/24",
					Interface:        "eth0",
					VirtualRouterID:  51,
					AdvertIntervalMs: tt.advertIntervalMs,
					CheckTCPPorts:    []int{443},
				},
			}

			output, err := RenderConfig(input)
			if err != nil {
				t.Fatalf("RenderConfig failed: %v", err)
			}

			// Check that the expected advert_int appears in the config
			expectedLine := fmt.Sprintf("advert_int %d", tt.expectedSeconds)
			if !strings.Contains(output, expectedLine) {
				t.Errorf("Config does not contain expected advert_int %d", tt.expectedSeconds)
			}
		})
	}
}

func TestRenderHealthScriptTCP_Deterministic(t *testing.T) {
	// Test that rendering the same ports multiple times produces identical output
	ports := []int{443, 8080, 9090}

	var outputs []string
	for i := 0; i < 10; i++ {
		output, err := RenderHealthScriptTCP(ports)
		if err != nil {
			t.Fatalf("RenderHealthScriptTCP failed on iteration %d: %v", i, err)
		}
		outputs = append(outputs, output)
	}

	// All outputs should be identical
	first := outputs[0]
	for i, output := range outputs {
		if output != first {
			t.Errorf("Output %d differs from first output", i)
		}
	}
}

func TestRenderHealthScriptTCP_AllPortsPresent(t *testing.T) {
	ports := []int{443, 8080, 9090}

	output, err := RenderHealthScriptTCP(ports)
	if err != nil {
		t.Fatalf("RenderHealthScriptTCP failed: %v", err)
	}

	// Check that all ports appear in the script
	for _, port := range ports {
		portStr := fmt.Sprintf(":%d", port)
		if !strings.Contains(output, portStr) {
			t.Errorf("Script does not contain port %d", port)
		}
	}

	// Check that the script has the correct shebang
	if !strings.HasPrefix(output, "#!/bin/bash") {
		t.Errorf("Script does not start with #!/bin/bash")
	}

	// Check that the script uses both ss and nc for redundancy
	if !strings.Contains(output, "ss -lnt") {
		t.Errorf("Script does not contain ss command")
	}
	if !strings.Contains(output, "nc -z") {
		t.Errorf("Script does not contain nc command")
	}
}

func TestRenderHealthScriptTCP_Sorted(t *testing.T) {
	// Test that ports are sorted in the output (deterministic)
	ports := []int{9090, 443, 8080}

	output, err := RenderHealthScriptTCP(ports)
	if err != nil {
		t.Fatalf("RenderHealthScriptTCP failed: %v", err)
	}

	// Find the position of each port in the output
	pos443 := strings.Index(output, "# Check port 443")
	pos8080 := strings.Index(output, "# Check port 8080")
	pos9090 := strings.Index(output, "# Check port 9090")

	// All positions should be found
	if pos443 == -1 || pos8080 == -1 || pos9090 == -1 {
		t.Fatalf("Not all ports found in output")
	}

	// Ports should appear in sorted order (443 < 8080 < 9090)
	if !(pos443 < pos8080 && pos8080 < pos9090) {
		t.Errorf("Ports are not in sorted order in the output")
	}
}

func TestRenderHealthScriptTCP_EmptyPorts(t *testing.T) {
	// Test that empty ports list returns an error
	ports := []int{}

	_, err := RenderHealthScriptTCP(ports)
	if err == nil {
		t.Errorf("Expected error for empty ports list, but got nil")
	}
}

func TestRenderConfig_Preempt(t *testing.T) {
	tests := []struct {
		name        string
		preempt     bool
		wantNoPreempt bool
	}{
		{
			name:          "Preempt enabled (default)",
			preempt:       true,
			wantNoPreempt: false,
		},
		{
			name:          "Preempt disabled (nopreempt)",
			preempt:       false,
			wantNoPreempt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RenderInput{
				NodeID:   "n1",
				Priority: 120,
				Spec: ingress.VIPFailoverSpec{
					VIP:              "10.0.0.250/24",
					Interface:        "eth0",
					VirtualRouterID:  51,
					AdvertIntervalMs: 1000,
					Preempt:          tt.preempt,
					CheckTCPPorts:    []int{443},
				},
			}

			output, err := RenderConfig(input)
			if err != nil {
				t.Fatalf("RenderConfig failed: %v", err)
			}

			hasNoPreempt := strings.Contains(output, "nopreempt")
			if hasNoPreempt != tt.wantNoPreempt {
				t.Errorf("Expected nopreempt in config: %v, but got: %v", tt.wantNoPreempt, hasNoPreempt)
				t.Logf("Config output:\n%s", output)
			}
		})
	}
}

func TestRenderConfig_BasicFields(t *testing.T) {
	// Test that all basic fields appear in the config
	input := RenderInput{
		NodeID:   "test-node",
		Priority: 150,
		Spec: ingress.VIPFailoverSpec{
			VIP:              "192.168.1.100/24",
			Interface:        "eth1",
			VirtualRouterID:  42,
			AdvertIntervalMs: 1500,
			CheckTCPPorts:    []int{443},
		},
	}

	output, err := RenderConfig(input)
	if err != nil {
		t.Fatalf("RenderConfig failed: %v", err)
	}

	// Check that all expected fields are present
	expectedFields := []string{
		"# Node: test-node",
		"# Priority: 150",
		"state BACKUP",
		"interface eth1",
		"virtual_router_id 42",
		"priority 150",
		"advert_int 1",
		"192.168.1.100/24",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Config does not contain expected field: %s", field)
		}
	}
}
