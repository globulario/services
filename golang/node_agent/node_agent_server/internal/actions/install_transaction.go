package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	InstallTxnPhaseStaging                = "staging"
	InstallTxnPhaseValidated              = "validated"
	InstallTxnPhasePromoted               = "promoted"
	InstallTxnPhaseReloaded               = "reloaded"
	InstallTxnPhaseCommitted              = "committed"
	InstallTxnPhaseRolledBack             = "rolled_back"
	InstallTxnPhasePartialInstallRecovery = "partial_install_recovery"
)

type InstallTransactionRecord struct {
	TransactionID        string            `json:"transaction_id"`
	NodeID               string            `json:"node_id,omitempty"`
	PackageID            string            `json:"package_id"`
	TargetBuildID        string            `json:"target_build_id,omitempty"`
	Phase                string            `json:"phase"`
	PreviousReceipt      map[string]string `json:"previous_receipt,omitempty"`
	PreviousFiles        []InstallTxnFile  `json:"previous_files,omitempty"`
	StagedPaths          []string          `json:"staged_paths,omitempty"`
	LastError            string            `json:"last_error,omitempty"`
	RecoveryInstructions []string          `json:"recovery_instructions,omitempty"`
	CreatedUnix          int64             `json:"created_unix"`
	UpdatedUnix          int64             `json:"updated_unix"`
}

type InstallTxnFile struct {
	Path       string `json:"path"`
	Existed    bool   `json:"existed"`
	SHA256     string `json:"sha256,omitempty"`
	BackupPath string `json:"backup_path,omitempty"`
}

type InstallTransactionError struct {
	TransactionID    string
	RecordPath       string
	RecoveryRequired bool
	Cause            error
}

func (e *InstallTransactionError) Error() string {
	if e == nil {
		return ""
	}
	if e.RecoveryRequired {
		return fmt.Sprintf("install transaction %s requires partial_install_recovery: %v", e.TransactionID, e.Cause)
	}
	return fmt.Sprintf("install transaction %s failed: %v", e.TransactionID, e.Cause)
}

func (e *InstallTransactionError) Unwrap() error { return e.Cause }

type installTxnPromotion struct {
	FinalPath  string
	StagedPath string
}

func installTransactionRoot() string {
	return filepath.Join(ActionStateDir, "install-transactions")
}

func installTransactionRecordPath(transactionID string) string {
	return filepath.Join(installTransactionRoot(), transactionID+".json")
}

func installTransactionWorkDir(transactionID string) string {
	return filepath.Join(installTransactionRoot(), transactionID)
}

func startInstallTransaction(rec *InstallTransactionRecord) error {
	if rec == nil || strings.TrimSpace(rec.TransactionID) == "" || strings.TrimSpace(rec.PackageID) == "" {
		return fmt.Errorf("install transaction requires transaction_id and package_id")
	}
	now := time.Now().Unix()
	rec.CreatedUnix = now
	rec.UpdatedUnix = now
	if rec.Phase == "" {
		rec.Phase = InstallTxnPhaseStaging
	}
	if err := os.MkdirAll(installTransactionWorkDir(rec.TransactionID), 0o755); err != nil {
		return err
	}
	return writeInstallTransaction(rec)
}

func writeInstallTransaction(rec *InstallTransactionRecord) error {
	if rec == nil {
		return fmt.Errorf("install transaction record is nil")
	}
	rec.UpdatedUnix = time.Now().Unix()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := installTransactionRecordPath(rec.TransactionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadInstallTransaction(transactionID string) (*InstallTransactionRecord, error) {
	data, err := os.ReadFile(installTransactionRecordPath(transactionID))
	if err != nil {
		return nil, err
	}
	var rec InstallTransactionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func updateInstallTransactionPhase(transactionID, phase, lastErr string) error {
	rec, err := loadInstallTransaction(transactionID)
	if err != nil {
		return err
	}
	rec.Phase = phase
	rec.LastError = strings.TrimSpace(lastErr)
	return writeInstallTransaction(rec)
}

func snapshotInstallTxnFile(transactionID, finalPath string) (InstallTxnFile, error) {
	info := InstallTxnFile{Path: finalPath}
	fi, err := os.Stat(finalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return info, nil
		}
		return info, err
	}
	if fi.IsDir() {
		return info, fmt.Errorf("snapshot %s: directories not supported", finalPath)
	}
	info.Existed = true
	info.SHA256, err = sha256File(finalPath)
	if err != nil {
		return info, err
	}
	backupDir := filepath.Join(installTransactionWorkDir(transactionID), "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return info, err
	}
	backupPath := filepath.Join(backupDir, strings.TrimPrefix(finalPath, "/"))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return info, err
	}
	if err := copyFile(finalPath, backupPath, fi.Mode().Perm()); err != nil {
		return info, err
	}
	info.BackupPath = backupPath
	return info, nil
}

func promoteInstallTransactionFiles(transactionID string, rec *InstallTransactionRecord, files []installTxnPromotion) error {
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if file.FinalPath == "" || file.StagedPath == "" {
			continue
		}
		if _, ok := seen[file.FinalPath]; ok {
			continue
		}
		seen[file.FinalPath] = struct{}{}
		snap, err := snapshotInstallTxnFile(transactionID, file.FinalPath)
		if err != nil {
			return err
		}
		rec.PreviousFiles = append(rec.PreviousFiles, snap)
		if err := os.MkdirAll(filepath.Dir(file.FinalPath), 0o755); err != nil {
			return err
		}
		if err := moveFile(file.StagedPath, file.FinalPath); err != nil {
			return err
		}
	}
	return writeInstallTransaction(rec)
}

func rollbackInstallTransactionByID(transactionID, reason string) (*InstallTransactionRecord, error) {
	rec, err := loadInstallTransaction(transactionID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rec.PreviousFiles, func(i, j int) bool {
		return rec.PreviousFiles[i].Path > rec.PreviousFiles[j].Path
	})
	var rollbackErrs []string
	for _, file := range rec.PreviousFiles {
		switch {
		case file.Existed:
			if file.BackupPath == "" {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s missing backup path", file.Path))
				continue
			}
			fi, err := os.Stat(file.BackupPath)
			if err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s backup unreadable: %v", file.Path, err))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s mkdir: %v", file.Path, err))
				continue
			}
			if err := copyFile(file.BackupPath, file.Path, fi.Mode().Perm()); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s restore: %v", file.Path, err))
			}
		default:
			if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s remove: %v", file.Path, err))
			}
		}
	}
	rec.LastError = strings.TrimSpace(reason)
	if len(rollbackErrs) > 0 {
		rec.Phase = InstallTxnPhasePartialInstallRecovery
		rec.RecoveryInstructions = []string{
			"Inspect the install transaction record and restore previous files from the recorded backup paths.",
			"Do not mark the package installed until the transaction is explicitly recovered or re-applied.",
		}
		_ = writeInstallTransaction(rec)
		return rec, errors.New(strings.Join(rollbackErrs, "; "))
	}
	rec.Phase = InstallTxnPhaseRolledBack
	rec.RecoveryInstructions = nil
	if err := writeInstallTransaction(rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func findInstallTransaction(packageID, targetBuildID string) (*InstallTransactionRecord, error) {
	entries, err := filepath.Glob(filepath.Join(installTransactionRoot(), "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	for i := len(entries) - 1; i >= 0; i-- {
		data, err := os.ReadFile(entries[i])
		if err != nil {
			continue
		}
		var rec InstallTransactionRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if strings.TrimSpace(rec.PackageID) != strings.TrimSpace(packageID) {
			continue
		}
		if targetBuildID != "" && strings.TrimSpace(rec.TargetBuildID) != strings.TrimSpace(targetBuildID) {
			continue
		}
		return &rec, nil
	}
	return nil, os.ErrNotExist
}

func RollbackActiveInstallTransaction(packageID, targetBuildID, reason string) (*InstallTransactionRecord, error) {
	rec, err := findInstallTransaction(packageID, targetBuildID)
	if err != nil {
		return nil, err
	}
	return rollbackInstallTransactionByID(rec.TransactionID, reason)
}

func CommitActiveInstallTransaction(packageID, targetBuildID string) error {
	rec, err := findInstallTransaction(packageID, targetBuildID)
	if err != nil {
		return err
	}
	rec.Phase = InstallTxnPhaseCommitted
	rec.LastError = ""
	rec.RecoveryInstructions = nil
	if err := writeInstallTransaction(rec); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(installTransactionWorkDir(rec.TransactionID), "backup"))
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := copyFile(src, dst, fi.Mode().Perm()); err != nil {
		return err
	}
	return os.Remove(src)
}
