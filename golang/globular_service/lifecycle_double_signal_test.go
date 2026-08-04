package globular_service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleManagerEntrypointsDoNotWaitForSecondSignal(t *testing.T) {
	root := filepath.Clean("..")
	var offenders []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "generated", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		if !strings.Contains(text, "NewLifecycleManager(") {
			return nil
		}
		idx := strings.Index(text, ".Start()")
		if idx < 0 {
			return nil
		}
		after := text[idx:]
		for _, forbidden := range []string{"signal.Notify", "syscall.SIGTERM", "<-sigChan", "GracefulShutdown"} {
			if strings.Contains(after, forbidden) {
				offenders = append(offenders, path+" contains "+forbidden+" after LifecycleManager.Start")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("LifecycleManager.Start already owns SIGTERM handling; entrypoints must not wait for a second shutdown signal:\n%s",
			strings.Join(offenders, "\n"))
	}
}
