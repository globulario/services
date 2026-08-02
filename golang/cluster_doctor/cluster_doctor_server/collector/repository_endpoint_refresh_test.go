package collector

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

type stubRepoClient struct {
	repopb.PackageRepositoryClient
}

func TestRefreshRepositoryClientClearsStartupEndpointMissing(t *testing.T) {
	c := New(CollectorConfig{}, nil)

	var calls int
	c.WithRepositoryClientResolver(func() (repopb.PackageRepositoryClient, bool) {
		calls++
		if calls == 1 {
			return nil, true
		}
		return stubRepoClient{}, false
	})

	c.refreshRepositoryClient()
	client, missing := c.repositoryState()
	if !missing {
		t.Fatalf("first refresh: endpoint should be marked missing")
	}
	if client != nil {
		t.Fatalf("first refresh: repository client must stay nil when endpoint is missing")
	}

	c.refreshRepositoryClient()
	client, missing = c.repositoryState()
	if missing {
		t.Fatalf("second refresh: endpoint_missing should clear once the service registers")
	}
	if client == nil {
		t.Fatalf("second refresh: repository client must be restored after registration")
	}
}

func TestRefreshRepositoryClientPreservesExistingClientOnTransientResolverFailure(t *testing.T) {
	c := New(CollectorConfig{}, nil)
	existing := stubRepoClient{}
	c.WithRepositoryClient(existing)

	c.WithRepositoryClientResolver(func() (repopb.PackageRepositoryClient, bool) {
		return nil, false
	})

	c.refreshRepositoryClient()
	client, missing := c.repositoryState()
	if missing {
		t.Fatalf("transient resolver failure must not masquerade as endpoint missing")
	}
	if client == nil {
		t.Fatalf("existing repository client should be preserved when resolver returns no replacement")
	}
}
