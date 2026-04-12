// verifier.go implements output verification strategies for compute jobs.
//
// Verification runs after output upload, before final job completion.
// The declared VerificationRule in the ComputeDefinition determines which
// verifier executes. Results map to explicit ResultTrustLevel values:
//
//   UNVERIFIED            — no verify_strategy declared or verification skipped
//   STRUCTURALLY_VERIFIED — output exists and has expected structure (size > 0)
//   CONTENT_VERIFIED      — CHECKSUM matched or schema validated
//   FULLY_REPRODUCED      — reserved for bitwise-identical reproduction
//
// Exit code alone is NOT sufficient for success. If verification is declared,
// the job terminal state depends on the verification result.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/compute/computepb"
	"google.golang.org/protobuf/types/known/structpb"
)

// verificationResult holds the outcome of a verification pass.
type verificationResult struct {
	Passed     bool
	TrustLevel computepb.ResultTrustLevel
	Checksums  []string
	Message    string
	Metadata   map[string]any
}

// verifyOutput runs the declared verification strategy against the unit's
// output. Returns a verificationResult that the caller should use to set
// the ComputeResult trust level and terminal state.
//
// If no verify_strategy is declared, returns UNVERIFIED (pass, no verification).
func verifyOutput(def *computepb.ComputeDefinition, stagingPath string, unit *computepb.ComputeUnit) verificationResult {
	rule := def.GetVerifyStrategy()
	if rule == nil || rule.Type == computepb.VerificationType_VERIFICATION_TYPE_UNSPECIFIED {
		return verificationResult{
			Passed:     true,
			TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
			Message:    "no verification strategy declared",
		}
	}

	switch rule.Type {
	case computepb.VerificationType_CHECKSUM:
		return verifyChecksum(rule, stagingPath, unit)
	case computepb.VerificationType_SCHEMA_VALIDATE:
		return verifyStructural(stagingPath)
	default:
		// Unsupported verification type — fall back to structural check.
		slog.Warn("compute verifier: unsupported verification type, falling back to structural",
			"type", rule.Type.String())
		return verifyStructural(stagingPath)
	}
}

// verifyChecksum compares the output checksum against expected values from the
// VerificationRule. The expected checksums come from rule.checks[] (each entry
// is a "sha256:<hex>" string or a bare hex string).
func verifyChecksum(rule *computepb.VerificationRule, stagingPath string, unit *computepb.ComputeUnit) verificationResult {
	outputDir := filepath.Join(stagingPath, "output")

	// Collect actual checksums from all output files.
	var actualChecksums []string
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		actualChecksums = append(actualChecksums, hex.EncodeToString(h.Sum(nil)))
		return nil
	})
	if err != nil {
		return verificationResult{
			Passed:     false,
			TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
			Message:    fmt.Sprintf("checksum verification failed: %v", err),
		}
	}

	if len(actualChecksums) == 0 {
		return verificationResult{
			Passed:     false,
			TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
			Message:    "no output files found for checksum verification",
		}
	}

	// Also include the unit's output ref checksum if available.
	if unit.GetOutputRef() != nil && unit.GetOutputRef().Sha256 != "" {
		actualChecksums = append(actualChecksums, unit.GetOutputRef().Sha256)
	}

	// Check against expected checksums from the rule.
	expectedChecks := rule.GetChecks()
	if len(expectedChecks) == 0 {
		// No expected checksums declared — structural pass only.
		slog.Info("compute verifier: CHECKSUM type but no expected values — structural pass",
			"actual_checksums", actualChecksums)
		return verificationResult{
			Passed:     true,
			TrustLevel: computepb.ResultTrustLevel_STRUCTURALLY_VERIFIED,
			Checksums:  actualChecksums,
			Message:    "checksum computed but no expected value to compare",
			Metadata:   map[string]any{"computed_checksums": actualChecksums},
		}
	}

	// Compare: every expected checksum must match at least one actual checksum.
	for _, expected := range expectedChecks {
		expected = normalizeChecksum(expected)
		matched := false
		for _, actual := range actualChecksums {
			if actual == expected {
				matched = true
				break
			}
		}
		if !matched {
			return verificationResult{
				Passed:     false,
				TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
				Checksums:  actualChecksums,
				Message:    fmt.Sprintf("checksum mismatch: expected %s not found in output", expected),
				Metadata: map[string]any{
					"expected":           expectedChecks,
					"computed_checksums": actualChecksums,
				},
			}
		}
	}

	slog.Info("compute verifier: CHECKSUM passed",
		"expected", expectedChecks, "actual", actualChecksums)
	return verificationResult{
		Passed:     true,
		TrustLevel: computepb.ResultTrustLevel_CONTENT_VERIFIED,
		Checksums:  actualChecksums,
		Message:    "all checksums matched",
		Metadata: map[string]any{
			"expected":           expectedChecks,
			"computed_checksums": actualChecksums,
		},
	}
}

// verifyStructural checks that the output directory exists and has at least
// one non-empty file. This is the minimum bar for STRUCTURALLY_VERIFIED.
func verifyStructural(stagingPath string) verificationResult {
	outputDir := filepath.Join(stagingPath, "output")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return verificationResult{
			Passed:     false,
			TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
			Message:    "output directory does not exist",
		}
	}

	hasFile := false
	filepath.Walk(outputDir, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() && info.Size() > 0 {
			hasFile = true
			return filepath.SkipAll
		}
		return nil
	})

	if !hasFile {
		return verificationResult{
			Passed:     false,
			TrustLevel: computepb.ResultTrustLevel_UNVERIFIED,
			Message:    "output directory is empty",
		}
	}

	return verificationResult{
		Passed:     true,
		TrustLevel: computepb.ResultTrustLevel_STRUCTURALLY_VERIFIED,
		Message:    "output exists with non-empty files",
	}
}

// normalizeChecksum strips common prefixes like "sha256:" from a checksum string.
func normalizeChecksum(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "sha256:")
	return strings.ToLower(s)
}

// verificationMetadataToStruct converts the metadata map to a protobuf Struct.
func verificationMetadataToStruct(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}
