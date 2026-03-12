package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/protobuf/proto"
)

const (
	signerKeyFile    = "/var/lib/globular/pki/plan-signer.key"
	signerPubFile    = "/var/lib/globular/pki/plan-signer.pub"
	signerKIDFile    = "/var/lib/globular/pki/plan-signer.kid"
	signerEtcdPrefix = "globular/security/plan-signers/"
	defaultPlanTTL   = 1 * time.Hour
)

type planSigner struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	kid        string
}

// allowUnsignedDispatch returns true if the controller is allowed to dispatch
// unsigned plans (compatibility/migration mode). Defaults to false (hardened).
// Set ALLOW_UNSIGNED_PLAN_DISPATCH=true ONLY for migration compatibility.
func allowUnsignedDispatch() bool {
	v := strings.TrimSpace(os.Getenv("ALLOW_UNSIGNED_PLAN_DISPATCH"))
	if v == "" {
		return false // default: hardened mode (v1 conformance)
	}
	return strings.EqualFold(v, "true") || v == "1"
}

// logPlanDispatchMode logs the current plan dispatch mode at startup.
func logPlanDispatchMode() {
	if allowUnsignedDispatch() {
		log.Printf("WARN plan-dispatch: running in COMPATIBILITY MODE (ALLOW_UNSIGNED_PLAN_DISPATCH=true) — unsigned plans may be dispatched on signing failure. Set ALLOW_UNSIGNED_PLAN_DISPATCH=false for production/v1 hardened mode.")
	} else {
		log.Printf("INFO plan-dispatch: running in HARDENED MODE — unsigned plans will NOT be dispatched on signing failure")
	}
}

// signOrAbort signs a plan and enforces the unsigned dispatch policy.
// In hardened mode (ALLOW_UNSIGNED_PLAN_DISPATCH=false), returns error on signing failure.
// In compatibility mode, logs a warning and returns nil so dispatch can proceed.
func (srv *server) signOrAbort(plan *planpb.NodePlan) error {
	if err := srv.signPlan(plan); err != nil {
		if !allowUnsignedDispatch() {
			log.Printf("ERROR plan=%s: signing failed, aborting dispatch (ALLOW_UNSIGNED_PLAN_DISPATCH=false): %v", plan.GetPlanId(), err)
			return fmt.Errorf("plan signing failed (hardened mode): %w", err)
		}
		log.Printf("WARN plan=%s: signing failed: %v (dispatching unsigned in compatibility mode)", plan.GetPlanId(), err)
	}
	return nil
}

// validateSigningKey checks that loaded key material is valid:
// - Ed25519 private key length is valid (64 bytes)
// - Ed25519 public key length is valid (32 bytes)
// - Derived public key from private key matches stored public key
// - KID is non-empty
func validateSigningKey(priv ed25519.PrivateKey, pub ed25519.PublicKey, kid string) error {
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key length: got %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length: got %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	derivedPub := priv.Public().(ed25519.PublicKey)
	if !bytes.Equal(derivedPub, pub) {
		return fmt.Errorf("public key does not match private key (keypair mismatch)")
	}
	kid = strings.TrimSpace(kid)
	if kid == "" {
		return fmt.Errorf("KID is empty")
	}
	return nil
}

// initPlanSigner loads or generates the cluster signing keypair.
// Public key is published to etcd for node-agent verification.
func (srv *server) initPlanSigner() error {
	// Try to load existing key
	if keyData, err := os.ReadFile(signerKeyFile); err == nil {
		pubData, err := os.ReadFile(signerPubFile)
		if err != nil {
			return fmt.Errorf("signing key exists but public key missing: %w", err)
		}
		kidData, err := os.ReadFile(signerKIDFile)
		if err != nil {
			return fmt.Errorf("signing key exists but kid missing: %w", err)
		}

		priv := ed25519.PrivateKey(keyData)
		pub := ed25519.PublicKey(pubData)
		kid := strings.TrimSpace(string(kidData))

		// Gap 5: Validate loaded key material
		if err := validateSigningKey(priv, pub, kid); err != nil {
			return fmt.Errorf("plan-signer: corrupted key material: %w", err)
		}

		srv.planSignerState = &planSigner{
			privateKey: priv,
			publicKey:  pub,
			kid:        kid,
		}
		log.Printf("plan-signer: loaded existing key (kid=%s)", srv.planSignerState.kid)
	} else {
		// Generate new keypair
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fmt.Errorf("generate signing key: %w", err)
		}
		// KID = "plan-v1-" + first 8 hex chars of SHA256(public key)
		h := sha256.Sum256(pub)
		kid := "plan-v1-" + hex.EncodeToString(h[:4])

		// Write files with strict permissions
		dir := filepath.Dir(signerKeyFile)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("create pki dir: %w", err)
		}
		if err := os.WriteFile(signerKeyFile, priv, 0600); err != nil {
			return fmt.Errorf("write signing key: %w", err)
		}
		if err := os.WriteFile(signerPubFile, pub, 0644); err != nil {
			return fmt.Errorf("write signing pub: %w", err)
		}
		if err := os.WriteFile(signerKIDFile, []byte(kid), 0644); err != nil {
			return fmt.Errorf("write signing kid: %w", err)
		}
		srv.planSignerState = &planSigner{privateKey: priv, publicKey: pub, kid: kid}
		log.Printf("plan-signer: generated new key (kid=%s)", kid)
	}

	// Publish public key to etcd trusted signer set
	if srv.etcdClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		key := signerEtcdPrefix + srv.planSignerState.kid

		// Gap 5: Check if etcd already contains a different public key for this KID
		existing, err := srv.etcdClient.Get(ctx, key)
		if err == nil && len(existing.Kvs) > 0 {
			etcdPub := ed25519.PublicKey(existing.Kvs[0].Value)
			if !bytes.Equal(etcdPub, srv.planSignerState.publicKey) {
				return fmt.Errorf("plan-signer: etcd already contains a DIFFERENT public key for kid=%s (local/etcd mismatch — possible key corruption or split-brain)", srv.planSignerState.kid)
			}
			log.Printf("plan-signer: etcd key matches local key (kid=%s)", srv.planSignerState.kid)
		} else {
			// Publish
			_, err := srv.etcdClient.Put(ctx, key, string(srv.planSignerState.publicKey))
			if err != nil {
				return fmt.Errorf("publish signing key to etcd: %w", err)
			}
			log.Printf("plan-signer: published public key to etcd (%s)", key)
		}
	}
	return nil
}

// signPlan signs a NodePlan using deterministic protobuf serialization.
// The signature covers all fields except the signature field itself.
// Also sets ExpiresUnixMs if not already set.
func (srv *server) signPlan(plan *planpb.NodePlan) error {
	if srv.planSignerState == nil {
		return fmt.Errorf("plan signer not initialized")
	}

	// Set expiry if not already set
	if plan.GetExpiresUnixMs() == 0 {
		plan.ExpiresUnixMs = uint64(time.Now().Add(defaultPlanTTL).UnixMilli())
	}

	// Clear signature field before serialization
	plan.Signature = nil

	// Deterministic marshal
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(plan)
	if err != nil {
		return fmt.Errorf("deterministic marshal for signing: %w", err)
	}

	// Sign with Ed25519
	sig := ed25519.Sign(srv.planSignerState.privateKey, data)

	// Set signature
	plan.Signature = &planpb.PlanSignature{
		Alg:   "EdDSA",
		KeyId: srv.planSignerState.kid,
		Sig:   sig,
	}
	return nil
}

// ensureNodeExecutorBinding creates an RBAC role binding for a node principal.
// Best-effort: logs warning on failure, does not block the caller.
func (srv *server) ensureNodeExecutorBinding(nodePrincipal string) {
	address, err := config.GetAddress()
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot resolve local address: %v", err)
		return
	}

	client, err := rbac_client.NewRbacService_Client(address, "rbac.RbacService")
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot connect to RBAC service: %v", err)
		return
	}
	defer client.Close()

	if err := client.SetRoleBinding(nodePrincipal, []string{security.RoleNodeExecutor}); err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: failed to set role binding for %s: %v", nodePrincipal, err)
		return
	}
	log.Printf("ensureNodeExecutorBinding: bound %s to role %s", nodePrincipal, security.RoleNodeExecutor)
}
