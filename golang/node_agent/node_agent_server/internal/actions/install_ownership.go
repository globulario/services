package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	InstallOwnershipStateRunning                = "running"
	InstallOwnershipStateCommitted              = "committed"
	InstallOwnershipStateRolledBack             = "rolled_back"
	InstallOwnershipStateReleased               = "released"
	InstallOwnershipStatePartialInstallRecovery = "partial_install_recovery"
	InstallOwnershipStateBlocked                = "blocked"

	InstallOwnershipModeNormal   = "normal"
	InstallOwnershipModeRecovery = "recovery"
)

const duplicateInstallCooldown = 10 * time.Second

type InstallOwnershipRecord struct {
	InstallKey        string `json:"install_key"`
	NodeID            string `json:"node_id"`
	PackageID         string `json:"package_id"`
	TargetBuildID     string `json:"target_build_id,omitempty"`
	TransactionID     string `json:"transaction_id,omitempty"`
	WorkflowRunID     string `json:"workflow_run_id,omitempty"`
	OperationID       string `json:"operation_id,omitempty"`
	State             string `json:"state"`
	Mode              string `json:"mode,omitempty"`
	LastResult        string `json:"last_result,omitempty"`
	LastError         string `json:"last_error,omitempty"`
	ConflictCount     int64  `json:"conflict_count,omitempty"`
	CooldownUntilUnix int64  `json:"cooldown_until_unix,omitempty"`
	AcquiredUnix      int64  `json:"acquired_unix"`
	UpdatedUnix       int64  `json:"updated_unix"`
	ReleasedUnix      int64  `json:"released_unix,omitempty"`
}

type AcquireInstallOwnershipRequest struct {
	NodeID        string
	PackageID     string
	TargetBuildID string
	TransactionID string
	WorkflowRunID string
	OperationID   string
	RecoveryMode  bool
}

type InstallOwnershipBusyError struct {
	Record *InstallOwnershipRecord
}

func (e *InstallOwnershipBusyError) Error() string {
	if e == nil || e.Record == nil {
		return "install ownership busy"
	}
	return fmt.Sprintf("install ownership busy for %s (owner=%s state=%s)",
		e.Record.InstallKey, e.Record.TransactionID, e.Record.State)
}

type InstallOwnershipCooldownError struct {
	Record        *InstallOwnershipRecord
	CooldownUntil time.Time
}

func (e *InstallOwnershipCooldownError) Error() string {
	if e == nil {
		return "install ownership cooling down"
	}
	return fmt.Sprintf("install ownership cooling down until %s", e.CooldownUntil.UTC().Format(time.RFC3339))
}

type InstallOwnershipRecoveryRequiredError struct {
	Record *InstallOwnershipRecord
}

func (e *InstallOwnershipRecoveryRequiredError) Error() string {
	if e == nil || e.Record == nil {
		return "partial_install_recovery blocks normal install"
	}
	return fmt.Sprintf("install ownership %s is in partial_install_recovery", e.Record.InstallKey)
}

var installOwnershipMu sync.Mutex
var installOwnershipSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func installOwnershipRoot() string {
	return filepath.Join(installTransactionRoot(), "ownership")
}

func installOwnershipKey(nodeID, packageID, targetBuildID string) string {
	return strings.Join([]string{
		strings.TrimSpace(nodeID),
		strings.TrimSpace(packageID),
		strings.TrimSpace(targetBuildID),
	}, "|")
}

func installOwnershipRecordPath(nodeID, packageID string) string {
	base := strings.TrimSpace(nodeID) + "__" + strings.TrimSpace(packageID)
	base = installOwnershipSanitizer.ReplaceAllString(base, "_")
	if base == "__" || base == "" {
		sum := sha256.Sum256([]byte(nodeID + "|" + packageID))
		base = hex.EncodeToString(sum[:8])
	}
	return filepath.Join(installOwnershipRoot(), base+".json")
}

func loadInstallOwnership(nodeID, packageID string) (*InstallOwnershipRecord, error) {
	data, err := os.ReadFile(installOwnershipRecordPath(nodeID, packageID))
	if err != nil {
		return nil, err
	}
	var rec InstallOwnershipRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func writeInstallOwnership(rec *InstallOwnershipRecord) error {
	if rec == nil {
		return fmt.Errorf("install ownership record is nil")
	}
	rec.UpdatedUnix = time.Now().Unix()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := installOwnershipRecordPath(rec.NodeID, rec.PackageID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func AcquireInstallOwnership(req AcquireInstallOwnershipRequest) (*InstallOwnershipRecord, error) {
	nodeID := strings.TrimSpace(req.NodeID)
	packageID := strings.TrimSpace(req.PackageID)
	transactionID := strings.TrimSpace(req.TransactionID)
	if nodeID == "" || packageID == "" || transactionID == "" {
		return nil, fmt.Errorf("install ownership requires node_id, package_id, and transaction_id")
	}

	installOwnershipMu.Lock()
	defer installOwnershipMu.Unlock()

	now := time.Now()
	installKey := installOwnershipKey(nodeID, packageID, req.TargetBuildID)
	rec, err := loadInstallOwnership(nodeID, packageID)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		rec = nil
	default:
		return nil, err
	}

	if rec != nil {
		if rec.TransactionID == transactionID && rec.State == InstallOwnershipStateRunning {
			return rec, nil
		}
		if rec.State == InstallOwnershipStatePartialInstallRecovery && !req.RecoveryMode {
			return nil, &InstallOwnershipRecoveryRequiredError{Record: rec}
		}
		if rec.State == InstallOwnershipStateRunning {
			rec.ConflictCount++
			rec.LastResult = "suppressed_duplicate"
			rec.LastError = fmt.Sprintf("install key owned by transaction %s", rec.TransactionID)
			rec.CooldownUntilUnix = now.Add(duplicateInstallCooldown).Unix()
			_ = writeInstallOwnership(rec)
			return nil, &InstallOwnershipBusyError{Record: rec}
		}
		if rec.InstallKey == installKey && rec.CooldownUntilUnix > now.Unix() && !req.RecoveryMode {
			return nil, &InstallOwnershipCooldownError{
				Record:        rec,
				CooldownUntil: time.Unix(rec.CooldownUntilUnix, 0),
			}
		}
	}

	acquired := &InstallOwnershipRecord{
		InstallKey:        installKey,
		NodeID:            nodeID,
		PackageID:         packageID,
		TargetBuildID:     strings.TrimSpace(req.TargetBuildID),
		TransactionID:     transactionID,
		WorkflowRunID:     strings.TrimSpace(req.WorkflowRunID),
		OperationID:       strings.TrimSpace(req.OperationID),
		State:             InstallOwnershipStateRunning,
		Mode:              InstallOwnershipModeNormal,
		LastResult:        "acquired",
		CooldownUntilUnix: 0,
		AcquiredUnix:      now.Unix(),
	}
	if req.RecoveryMode {
		acquired.Mode = InstallOwnershipModeRecovery
	}
	if rec != nil {
		acquired.ConflictCount = rec.ConflictCount
	}
	if err := writeInstallOwnership(acquired); err != nil {
		return nil, err
	}
	return acquired, nil
}

func CloseInstallOwnership(nodeID, packageID, targetBuildID, transactionID, state, lastErr string, cooldown time.Duration) error {
	nodeID = strings.TrimSpace(nodeID)
	packageID = strings.TrimSpace(packageID)
	if nodeID == "" || packageID == "" {
		return fmt.Errorf("install ownership close requires node_id and package_id")
	}

	installOwnershipMu.Lock()
	defer installOwnershipMu.Unlock()

	rec, err := loadInstallOwnership(nodeID, packageID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(transactionID) != "" && strings.TrimSpace(rec.TransactionID) != strings.TrimSpace(transactionID) {
		return fmt.Errorf("install ownership close transaction mismatch: have %s want %s", rec.TransactionID, transactionID)
	}
	if targetBuildID != "" && strings.TrimSpace(rec.TargetBuildID) != strings.TrimSpace(targetBuildID) {
		return fmt.Errorf("install ownership close build mismatch: have %s want %s", rec.TargetBuildID, targetBuildID)
	}
	rec.State = strings.TrimSpace(state)
	rec.LastResult = rec.State
	rec.LastError = strings.TrimSpace(lastErr)
	rec.ReleasedUnix = time.Now().Unix()
	rec.TransactionID = strings.TrimSpace(transactionID)
	if cooldown > 0 {
		rec.CooldownUntilUnix = time.Now().Add(cooldown).Unix()
	} else if rec.State != InstallOwnershipStateRunning {
		rec.CooldownUntilUnix = 0
	}
	return writeInstallOwnership(rec)
}
