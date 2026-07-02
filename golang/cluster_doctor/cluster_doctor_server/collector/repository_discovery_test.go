package collector

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestRefreshRepositoryDiscoveryClearsLatchedMissingFlagWhenEndpointAppears(t *testing.T) {
	prevResolve := resolveServiceAddrFn
	prevDial := newRepositoryClientFn
	t.Cleanup(func() {
		resolveServiceAddrFn = prevResolve
		newRepositoryClientFn = prevDial
	})

	resolveServiceAddrFn = func(serviceName, fallback string) string {
		if serviceName != "repository.PackageRepository" {
			t.Fatalf("unexpected service lookup: %s", serviceName)
		}
		return "10.0.0.63:10004"
	}

	dialed := 0
	newRepositoryClientFn = func(endpoint string) (repopb.PackageRepositoryClient, error) {
		dialed++
		if endpoint != "10.0.0.63:10004" {
			t.Fatalf("unexpected repository endpoint: %s", endpoint)
		}
		return nil, nil
	}

	c := &Collector{repoEndpointMissing: true}
	c.refreshRepositoryDiscovery()

	if c.repoEndpointMissing {
		t.Fatal("repoEndpointMissing should clear when repository endpoint is present in etcd")
	}
	if dialed != 1 {
		t.Fatalf("expected one late-bind attempt, got %d", dialed)
	}
}

func TestRefreshRepositoryDiscoveryKeepsMissingFlagWhenEndpointAbsent(t *testing.T) {
	prevResolve := resolveServiceAddrFn
	prevDial := newRepositoryClientFn
	t.Cleanup(func() {
		resolveServiceAddrFn = prevResolve
		newRepositoryClientFn = prevDial
	})

	resolveServiceAddrFn = func(serviceName, fallback string) string {
		if serviceName != "repository.PackageRepository" {
			t.Fatalf("unexpected service lookup: %s", serviceName)
		}
		return ""
	}

	dialed := 0
	newRepositoryClientFn = func(endpoint string) (repopb.PackageRepositoryClient, error) {
		dialed++
		return nil, nil
	}

	c := &Collector{}
	c.refreshRepositoryDiscovery()

	if !c.repoEndpointMissing {
		t.Fatal("repoEndpointMissing should stay true when repository endpoint is absent")
	}
	if dialed != 0 {
		t.Fatalf("expected no late-bind attempt when endpoint is absent, got %d", dialed)
	}
}
