package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	configpkg "github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
)

var (
	doctorBaseline bool
	doctorEnvoy    bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run baseline or Envoy-specific health checks",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorBaseline, "baseline", false, "Run the runtime baseline checks (default)")
	doctorCmd.Flags().BoolVar(&doctorEnvoy, "envoy", false, "Run the Envoy/xDS validation checks")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	if !doctorBaseline && !doctorEnvoy {
		doctorBaseline = true
	}
	var failures []error
	if doctorBaseline {
		if err := runBaselineChecks(); err != nil {
			failures = append(failures, fmt.Errorf("baseline: %w", err))
		} else {
			fmt.Println("Result: OK (baseline)")
		}
	}
	if doctorEnvoy {
		if err := runEnvoyChecks(); err != nil {
			failures = append(failures, fmt.Errorf("envoy: %w", err))
		} else {
			fmt.Println("Result: OK (envoy)")
		}
	}
	if len(failures) > 0 {
		for _, err := range failures {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		}
		return errors.New("doctor checks failed")
	}
	return nil
}

func runBaselineChecks() error {
	var missing []string
	dirs := []string{
		"/run/globular",
		"/var/lib/globular",
		"/var/lib/globular/config",
		"/var/log/globular",
	}
	for _, dir := range dirs {
		if err := ensureRuntimeDir(dir); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%v)", dir, err))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing runtime directories: %s", strings.Join(missing, ", "))
	}
	if err := ensureLocalConfig(); err != nil {
		return fmt.Errorf("local config missing %s: %w", configpkg.GetRuntimeConfigPath(), err)
	}
	services := []string{"globular-node-agent.service", "globular-gateway.service"}
	for _, svc := range services {
		if err := assertServiceActive(svc); err != nil {
			return err
		}
	}
	return nil
}

func runEnvoyChecks() error {
	services := []string{"globular-envoy.service", "globular-xds.service"}
	for _, svc := range services {
		if err := assertServiceActive(svc); err != nil {
			return err
		}
	}
	return nil
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not directory")
	}
	return nil
}

func ensureRuntimeDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if u, err := user.Lookup("globular"); err == nil {
		uid, gid := parseUID(u.Uid), parseUID(u.Gid)
		if uid >= 0 && gid >= 0 {
			if err := os.Chown(path, uid, gid); err != nil && !os.IsPermission(err) {
				return fmt.Errorf("chown %s: %w", path, err)
			}
		}
	}
	return nil
}

func parseUID(val string) int {
	id, err := strconv.Atoi(val)
	if err != nil {
		return -1
	}
	return id
}

func assertServiceActive(name string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl missing: %w", err)
	}
	cmd := exec.Command("systemctl", "is-active", "--quiet", name)
	if err := cmd.Run(); err != nil {
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "reset-failed", name).Run()
		_ = exec.Command("systemctl", "restart", name).Run()
		time.Sleep(500 * time.Millisecond)
		if err2 := exec.Command("systemctl", "is-active", "--quiet", name).Run(); err2 != nil {
			return fmt.Errorf("service %s not active: %w", name, err2)
		}
	}
	return nil
}

func ensureLocalConfig() error {
	path := configpkg.GetRuntimeConfigPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := ensureRuntimeDir(configpkg.GetRuntimeConfigDir()); err != nil {
		return err
	}
	src := "/usr/share/globular/defaults/config.json"
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
