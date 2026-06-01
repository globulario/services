// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=repository_signing_commands
// @awareness implements=globular.platform:intent.repository.signature_policy_gates_trust
// @awareness risk=high
package main

// repo_signing_cmds.go — Phase CLI-B: signature + trusted-publisher commands.
//
// CLI surface:
//   globular repository trust-publisher <publisher_id> --key <public-key-file>
//   globular repository revoke-publisher-key <publisher_id> --key-id <key-id>
//   globular repository trusted-publishers
//   globular repository signature verify <publisher/name> <version>
//   globular repository signature list   <publisher/name> <version>
//   globular repository signature sign   <publisher/name> <version> --signature <bytes-file> --key-id <id>
//
// Private signing keys NEVER leave the local filesystem. The CLI signs
// locally with `globular repository signature sign` after the operator
// produces the detached signature with their preferred ed25519 tool — the
// repository only stores the public key + signature bytes.

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── trust-publisher / revoke-publisher-key / trusted-publishers ────────────

var (
	trustKeyFile   string
	trustKeyID     string
	trustValidUntil int64
	trustNotes     string
	trustJSON      bool
)

var repoTrustPublisherCmd = &cobra.Command{
	Use:   "trust-publisher <publisher_id>",
	Short: "Register a trusted publisher's public key (admin only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		publisherID := args[0]
		if trustKeyFile == "" {
			return fmt.Errorf("--key <public-key-file> is required")
		}
		pemBytes, err := os.ReadFile(trustKeyFile)
		if err != nil {
			return fmt.Errorf("read key file: %w", err)
		}
		keyID := trustKeyID
		if keyID == "" {
			keyID = strings.TrimSuffix(filepathBase(trustKeyFile), filepathExt(trustKeyFile))
		}

		client, err := newRepoClient()
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.TrustPublisher(&repopb.TrustPublisherRequest{
			PublisherId:    publisherID,
			PublicKeyId:    keyID,
			PublicKeyPem:   pemBytes,
			Algorithm:      "ed25519",
			ValidUntilUnix: trustValidUntil,
			Notes:          trustNotes,
		})
		if err != nil {
			return fmt.Errorf("trust-publisher: %w", err)
		}
		if trustJSON {
			emitJSON(resp.GetPublisher())
			return nil
		}
		fmt.Printf("trusted publisher: %s key=%s state=%s\n",
			publisherID, keyID, resp.GetPublisher().GetTrustState())
		return nil
	},
}

var (
	revokeKeyID  string
	revokeReason string
)

var repoRevokeKeyCmd = &cobra.Command{
	Use:   "revoke-publisher-key <publisher_id>",
	Short: "Revoke a trusted publisher key (terminal — admin only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if revokeKeyID == "" {
			return fmt.Errorf("--key-id <key-id> is required")
		}
		client, err := newRepoClient()
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.RevokePublisherKey(&repopb.RevokePublisherKeyRequest{
			PublisherId:  args[0],
			PublicKeyId:  revokeKeyID,
			Reason:       revokeReason,
		})
		if err != nil {
			return fmt.Errorf("revoke-publisher-key: %w", err)
		}
		fmt.Printf("revoked: %s key=%s state=%s\n",
			resp.GetPublisher().GetPublisherId(),
			resp.GetPublisher().GetPublicKeyId(),
			resp.GetPublisher().GetTrustState())
		return nil
	},
}

var (
	listTrustedFilter string
	listTrustedJSON   bool
)

var repoTrustedPublishersCmd = &cobra.Command{
	Use:   "trusted-publishers",
	Short: "List trusted publishers and their key trust state",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newRepoClient()
		if err != nil {
			return err
		}
		defer client.Close()
		resp, err := client.ListTrustedPublishers(&repopb.ListTrustedPublishersRequest{
			PublisherId: listTrustedFilter,
		})
		if err != nil {
			return err
		}
		if listTrustedJSON {
			emitJSON(resp.GetPublishers())
			return nil
		}
		fmt.Printf("%-32s %-24s %-10s %-12s %s\n",
			"PUBLISHER", "KEY_ID", "ALGO", "STATE", "VALID_UNTIL")
		for _, p := range resp.GetPublishers() {
			vu := "—"
			if p.GetValidUntilUnix() > 0 {
				vu = fmt.Sprintf("%d", p.GetValidUntilUnix())
			}
			fmt.Printf("%-32s %-24s %-10s %-12s %s\n",
				truncStrDup(p.GetPublisherId(), 32), truncStrDup(p.GetPublicKeyId(), 24),
				truncStrDup(p.GetAlgorithm(), 10),
				strings.TrimPrefix(p.GetTrustState().String(), "TRUST_"), vu)
		}
		return nil
	},
}

// ── signature verify / list / sign ────────────────────────────────────────

var (
	sigPlatform    string
	sigBuildNumber int64
	sigKind        string
	sigJSON        bool
	signSigFile    string
	signKeyID      string
	signProvenance string
)

var repoSignatureCmd = &cobra.Command{
	Use:   "signature",
	Short: "Artifact signature commands (verify | list | sign)",
}

var repoSignatureVerifyCmd = &cobra.Command{
	Use:   "verify <publisher/name> <version>",
	Short: "Verify the most recent signature on an artifact against trusted publishers",
	Args:  cobra.ExactArgs(2),
	RunE:  runSignatureVerify,
}

var repoSignatureListCmd = &cobra.Command{
	Use:   "list <publisher/name> <version>",
	Short: "List every signature recorded for an artifact",
	Args:  cobra.ExactArgs(2),
	RunE:  runSignatureList,
}

var repoSignatureSignCmd = &cobra.Command{
	Use:   "sign <publisher/name> <version>",
	Short: "Register a detached signature for an artifact (signature already produced locally)",
	Long: `Registers a detached ed25519 signature for an artifact's digest.

Sign the artifact digest with your private key locally, then provide the
resulting signature bytes via --signature <file> and the public key id
via --key-id. Globular never sees your private key.

To produce the signature with openssl on a digest "sha256:abc..."::

    printf 'sha256:abc...' | openssl pkeyutl -sign -inkey ed25519.key > sig.bin

Then::

    globular repository signature sign core@globular.io/echo 1.0.0 \
      --signature sig.bin --key-id core-prod-2026`,
	Args: cobra.ExactArgs(2),
	RunE: runSignatureSign,
}

func runSignatureVerify(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.VerifyArtifactSignature(&repopb.VerifyArtifactSignatureRequest{
		Ref: &repopb.ArtifactRef{
			PublisherId: publisher, Name: name, Version: args[1],
			Platform: sigPlatform, Kind: resolveArtifactKind(sigKind),
		},
		BuildNumber: sigBuildNumber,
	})
	if err != nil {
		return err
	}
	if sigJSON {
		emitJSON(resp)
	} else {
		fmt.Printf("status: %s\n", strings.TrimPrefix(resp.GetStatus().String(), "SIGNATURE_"))
		fmt.Printf("reason: %s\n", resp.GetReason())
		if sig := resp.GetSignature(); sig != nil {
			fmt.Printf("key_id: %s\n", sig.GetPublicKeyId())
			fmt.Printf("signed_by: %s\n", sig.GetSignedBy())
			fmt.Printf("signed_at_unix: %d\n", sig.GetSignedAtUnix())
		}
	}
	if resp.GetStatus() != repopb.SignatureStatus_SIGNATURE_OK {
		os.Exit(2)
	}
	return nil
}

func runSignatureList(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()
	resp, err := client.ListArtifactSignatures(&repopb.ListArtifactSignaturesRequest{
		Ref: &repopb.ArtifactRef{
			PublisherId: publisher, Name: name, Version: args[1],
			Platform: sigPlatform, Kind: resolveArtifactKind(sigKind),
		},
		BuildNumber: sigBuildNumber,
	})
	if err != nil {
		return err
	}
	if sigJSON {
		emitJSON(resp.GetSignatures())
		return nil
	}
	fmt.Printf("%-24s %-10s %-32s %s\n", "KEY_ID", "ALGO", "SIGNED_BY", "SIGNED_AT")
	for _, s := range resp.GetSignatures() {
		fmt.Printf("%-24s %-10s %-32s %d\n",
			truncStrDup(s.GetPublicKeyId(), 24), truncStrDup(s.GetAlgorithm(), 10),
			truncStrDup(s.GetSignedBy(), 32), s.GetSignedAtUnix())
	}
	return nil
}

func runSignatureSign(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	if signSigFile == "" || signKeyID == "" {
		return fmt.Errorf("--signature <file> and --key-id are required")
	}
	sigBytes, err := os.ReadFile(signSigFile)
	if err != nil {
		return fmt.Errorf("read signature file: %w", err)
	}
	if len(sigBytes) == 0 {
		return fmt.Errorf("signature file is empty")
	}

	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()
	resp, err := client.RegisterArtifactSignature(&repopb.RegisterArtifactSignatureRequest{
		Ref: &repopb.ArtifactRef{
			PublisherId: publisher, Name: name, Version: args[1],
			Platform: sigPlatform, Kind: resolveArtifactKind(sigKind),
		},
		BuildNumber:    sigBuildNumber,
		Algorithm:      "ed25519",
		PublicKeyId:    signKeyID,
		SignatureBytes: sigBytes,
		ProvenanceRef:  signProvenance,
	})
	if err != nil {
		return err
	}
	if sigJSON {
		emitJSON(resp)
	} else {
		fmt.Printf("registered signature for %s\n", resp.GetSignature().GetArtifactKey())
		fmt.Printf("status: %s\n", strings.TrimPrefix(resp.GetStatus().String(), "SIGNATURE_"))
	}
	if resp.GetStatus() != repopb.SignatureStatus_SIGNATURE_OK {
		os.Exit(2)
	}
	return nil
}

// ── Tiny path helpers (avoid pulling path/filepath where one fn is enough) ─

func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func filepathExt(p string) string {
	base := filepathBase(p)
	if i := strings.LastIndex(base, "."); i >= 0 {
		return base[i:]
	}
	return ""
}

// ── init: register all signing/trust commands under repoCmd ────────────────

func init() {
	defaultPlatform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	repoTrustPublisherCmd.Flags().StringVar(&trustKeyFile, "key", "", "Path to PEM-encoded public key file (required)")
	repoTrustPublisherCmd.Flags().StringVar(&trustKeyID, "key-id", "", "Operator-supplied identifier (default: filename without extension)")
	repoTrustPublisherCmd.Flags().Int64Var(&trustValidUntil, "valid-until", 0, "Optional unix timestamp after which key is EXPIRED (0 = never)")
	repoTrustPublisherCmd.Flags().StringVar(&trustNotes, "notes", "", "Optional free-form note")
	repoTrustPublisherCmd.Flags().BoolVar(&trustJSON, "json", false, "Emit JSON output")

	repoRevokeKeyCmd.Flags().StringVar(&revokeKeyID, "key-id", "", "Key id to revoke (required)")
	repoRevokeKeyCmd.Flags().StringVar(&revokeReason, "reason", "", "Reason recorded for audit")

	repoTrustedPublishersCmd.Flags().StringVar(&listTrustedFilter, "publisher", "", "Filter to one publisher_id (default: all)")
	repoTrustedPublishersCmd.Flags().BoolVar(&listTrustedJSON, "json", false, "Emit JSON output")

	for _, c := range []*cobra.Command{repoSignatureVerifyCmd, repoSignatureListCmd, repoSignatureSignCmd} {
		c.Flags().StringVar(&sigPlatform, "platform", defaultPlatform, "Target platform")
		c.Flags().Int64Var(&sigBuildNumber, "build-number", 0, "Specific build (0 = latest PUBLISHED)")
		c.Flags().StringVar(&sigKind, "kind", "service", "Artifact kind")
		c.Flags().BoolVar(&sigJSON, "json", false, "Emit JSON output")
	}
	repoSignatureSignCmd.Flags().StringVar(&signSigFile, "signature", "", "Path to detached signature bytes (required)")
	repoSignatureSignCmd.Flags().StringVar(&signKeyID, "key-id", "", "Public key id used to sign (required)")
	repoSignatureSignCmd.Flags().StringVar(&signProvenance, "provenance", "", "Optional provenance reference")

	repoSignatureCmd.AddCommand(repoSignatureVerifyCmd, repoSignatureListCmd, repoSignatureSignCmd)
	repoCmd.AddCommand(repoTrustPublisherCmd, repoRevokeKeyCmd, repoTrustedPublishersCmd, repoSignatureCmd)
}
