// Package approvaltest installs an in-process Ed25519 keystore so tests
// in other packages can mint and validate approval tokens without touching
// /etc/globular/config or the file keystore. Import it only from _test.go
// files.
// @awareness namespace=globular.platform
// @awareness component=platform_security
// @awareness file_role=test_keyring_helper
// @awareness risk=low
package approvaltest

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/globulario/services/golang/security"
)

// Defaults installed by Install when its arguments are empty.
const (
	DefaultIssuer    = "00:11:22:33:44:55"
	DefaultClusterID = "test.cluster.local"
	DefaultKID       = "approvaltest-kid"
)

// Install wires an Ed25519 keypair into the security package's issuer/
// peer-key callbacks and overrides the approval-token issuer + cluster id
// lookups. It registers cleanup via t.Cleanup so other tests see the
// pre-call values.
//
// Pass empty strings for issuer/clusterID to accept the defaults.
func Install(t *testing.T, issuer, clusterID string) {
	t.Helper()
	if issuer == "" {
		issuer = DefaultIssuer
	}
	if clusterID == "" {
		clusterID = DefaultClusterID
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("approvaltest: generate ed25519 key: %v", err)
	}

	prevSign := security.GetIssuerSigningKey
	prevVerify := security.GetPeerPublicKey
	security.GetIssuerSigningKey = func(iss string) (ed25519.PrivateKey, string, error) {
		if iss != issuer {
			return nil, "", errors.New("approvaltest: unexpected issuer: " + iss)
		}
		return priv, DefaultKID, nil
	}
	security.GetPeerPublicKey = func(iss, kid string) (ed25519.PublicKey, error) {
		if iss != issuer {
			return nil, errors.New("approvaltest: unexpected issuer: " + iss)
		}
		if kid != "" && kid != DefaultKID {
			return nil, errors.New("approvaltest: unexpected kid: " + kid)
		}
		return pub, nil
	}

	restoreIssuer := security.SetApprovalIssuerForTesting(func() (string, error) { return issuer, nil })
	restoreCluster := security.SetApprovalClusterIDForTesting(func() (string, error) { return clusterID, nil })

	t.Cleanup(func() {
		restoreIssuer()
		restoreCluster()
		security.GetIssuerSigningKey = prevSign
		security.GetPeerPublicKey = prevVerify
	})
}
