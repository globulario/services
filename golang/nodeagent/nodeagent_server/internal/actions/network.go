package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"google.golang.org/protobuf/types/known/structpb"
)

type networkApplyAction struct{}

func (networkApplyAction) Name() string { return "network.apply_spec" }

func (networkApplyAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	spec := strings.TrimSpace(args.GetFields()["spec_json"].GetStringValue())
	if spec == "" {
		spec = strings.TrimSpace(args.GetFields()["spec"].GetStringValue())
	}
	if spec == "" {
		return errors.New("spec_json is required")
	}
	return nil
}

func (networkApplyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	spec := strings.TrimSpace(fields["spec_json"].GetStringValue())
	if spec == "" {
		spec = strings.TrimSpace(fields["spec"].GetStringValue())
	}
	if spec == "" {
		return "", errors.New("spec_json is required")
	}
	mode := strings.ToLower(strings.TrimSpace(fields["mode"].GetStringValue()))
	if mode == "" {
		mode = "merge"
	}
	snapshotPath := filepath.Join(config.GetRuntimeConfigDir(), "cluster_network_spec.json")
	if err := writeAtomicIfChanged(snapshotPath, []byte(spec), 0o600); err != nil {
		return "", fmt.Errorf("write network spec snapshot: %w", err)
	}

	target := "/var/lib/globular/network.json"
	overlay, err := decodeNetworkOverlayFromProtoJSON([]byte(spec))
	if err != nil {
		return "", fmt.Errorf("decode network overlay: %w", err)
	}

	switch mode {
	case "replace":
		if err := replaceNetworkConfig(target, overlay); err != nil {
			return "", fmt.Errorf("replace network config: %w", err)
		}
		return "network config replaced", nil
	default:
		if err := mergeNetworkIntoConfig(target, overlay); err != nil {
			return "", fmt.Errorf("merge network config: %w", err)
		}
		return "network config applied", nil
	}
}

func mergeNetworkIntoConfig(target string, overlay map[string]interface{}) error {
	if len(overlay) == 0 {
		return nil
	}
	base := make(map[string]interface{})
	if data, err := os.ReadFile(target); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &base); err != nil {
			return fmt.Errorf("parse base config: %w", err)
		}
	}
	if base == nil {
		base = make(map[string]interface{})
	}
	changed := false
	for key, value := range overlay {
		if _, ok := allowedNetworkKeys[key]; ok {
			if existing, ok := base[key]; !ok || !deepEqual(existing, value) {
				changed = true
			}
			base[key] = value
		}
	}
	if !changed {
		return nil
	}
	merged, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal merged config: %w", err)
	}
	if err := writeAtomicIfChanged(target, merged, 0o644); err != nil {
		return fmt.Errorf("write merged config: %w", err)
	}
	return nil
}

func replaceNetworkConfig(target string, overlay map[string]interface{}) error {
	if len(overlay) == 0 {
		return nil
	}
	dest := make(map[string]interface{})
	for key, value := range overlay {
		if _, ok := allowedNetworkKeys[key]; ok {
			dest[key] = value
		}
	}
	data, err := json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal replaced config: %w", err)
	}
	return writeAtomicIfChanged(target, data, 0o644)
}

var allowedNetworkKeys = map[string]struct{}{
	"Domain":           {},
	"Protocol":         {},
	"PortHTTP":         {},
	"PortHTTPS":        {},
	"ACMEEnabled":      {},
	"AdminEmail":       {},
	"AlternateDomains": {},
}

func decodeNetworkOverlayFromProtoJSON(specJSON []byte) (map[string]interface{}, error) {
	if len(specJSON) == 0 {
		return nil, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(specJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse overlay json: %w", err)
	}
	canonical := make(map[string]interface{})
	for key, val := range raw {
		switch strings.TrimSpace(key) {
		case "clusterDomain":
			canonical["Domain"] = val
		case "protocol":
			canonical["Protocol"] = val
		case "portHttp":
			canonical["PortHTTP"] = val
		case "portHttps":
			canonical["PortHTTPS"] = val
		case "acmeEnabled":
			canonical["ACMEEnabled"] = val
		case "adminEmail":
			canonical["AdminEmail"] = val
		case "alternateDomains":
			canonical["AlternateDomains"] = val
		default:
			// Allow canonical keys to pass through unchanged
			if _, ok := allowedNetworkKeys[key]; ok {
				canonical[key] = val
			}
		}
	}
	return canonical, nil
}

func writeAtomicIfChanged(dest string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if existing, err := os.ReadFile(dest); err == nil {
		oldHash := sha256.Sum256(existing)
		newHash := sha256.Sum256(data)
		if oldHash == newHash {
			return nil
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func deepEqual(a, b interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return hex.EncodeToString(aj) == hex.EncodeToString(bj)
}

func init() {
	Register(networkApplyAction{})
}
