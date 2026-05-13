package main

import "testing"

func recordValues(state *DesiredDNSState, name string, typ RecordType) []string {
	out := make([]string, 0)
	for _, r := range state.Records {
		if r.Name == name && r.Type == typ {
			out = append(out, r.Value)
		}
	}
	return out
}

func containsValue(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestDNSDesiredState_DNSRecordOnlyIncludesDNSProfileNodes(t *testing.T) {
	nodes := []NodeInfo{
		{FQDN: "core-1.globular.internal", IPv4: "10.0.0.1", Profiles: []string{"core", "dns", "gateway"}},
		{FQDN: "worker-1.globular.internal", IPv4: "10.0.0.2", Profiles: []string{"worker"}},
		{FQDN: "db-1.globular.internal", IPv4: "10.0.0.3", Profiles: []string{"scylla"}},
	}
	state := ComputeDesiredState("globular.internal", nodes, 1)
	got := recordValues(state, "dns.globular.internal", RecordTypeA)
	if len(got) != 1 || !containsValue(got, "10.0.0.1") {
		t.Fatalf("dns record should only contain dns/core profile node, got=%v", got)
	}
}

func TestDNSDesiredState_GatewayRecordOnlyIncludesGatewayProfileNodes(t *testing.T) {
	nodes := []NodeInfo{
		{FQDN: "core-1.globular.internal", IPv4: "10.0.0.1", Profiles: []string{"core", "dns", "gateway"}},
		{FQDN: "worker-1.globular.internal", IPv4: "10.0.0.2", Profiles: []string{"worker"}},
		{FQDN: "db-1.globular.internal", IPv4: "10.0.0.3", Profiles: []string{"scylla"}},
	}
	state := ComputeDesiredState("globular.internal", nodes, 1)
	got := recordValues(state, "gateway.globular.internal", RecordTypeA)
	if len(got) != 1 || !containsValue(got, "10.0.0.1") {
		t.Fatalf("gateway record should only contain gateway profile nodes, got=%v", got)
	}
}

func TestDNSDesiredState_DoesNotPublishJoinerAsDNS(t *testing.T) {
	nodes := []NodeInfo{
		{FQDN: "core-1.globular.internal", IPv4: "10.0.0.1", Profiles: []string{"core", "dns"}},
		{FQDN: "joiner-1.globular.internal", IPv4: "10.0.0.44", Profiles: []string{"worker"}},
	}
	state := ComputeDesiredState("globular.internal", nodes, 1)
	got := recordValues(state, "dns.globular.internal", RecordTypeA)
	if containsValue(got, "10.0.0.44") {
		t.Fatalf("dns record must not include non-dns joiner node, got=%v", got)
	}
}

