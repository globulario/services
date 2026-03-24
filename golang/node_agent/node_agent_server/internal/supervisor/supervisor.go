package supervisor

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var allowed = map[string]struct{}{
	"start":   {},
	"stop":    {},
	"restart": {},
	"enable":  {},
	"disable": {},
	"status":  {},
}

// ApplyUnitAction executes the requested action for the given unit via systemctl.
func ApplyUnitAction(ctx context.Context, unitName, action string) (string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if unitName == "" {
		return "", errors.New("unit name is required")
	}
	if _, ok := allowed[action]; !ok {
		return "", errors.New("unsupported action: " + action)
	}

	cmd := exec.CommandContext(ctx, "systemctl", action, unitName)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runSystemctl(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// DaemonReload reloads systemd unit files.
func DaemonReload(ctx context.Context) error {
	_, err := runSystemctl(ctx, "daemon-reload")
	return err
}

// IsActive returns true when a unit reports active.
func IsActive(ctx context.Context, unit string) (bool, error) {
	if unit == "" {
		return false, errors.New("unit name is required")
	}
	err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

// IsLoaded returns true when a unit file is loaded by systemd (regardless of active state).
func IsLoaded(ctx context.Context, unit string) (bool, error) {
	if unit == "" {
		return false, errors.New("unit name is required")
	}
	out, err := exec.CommandContext(ctx, "systemctl", "show", "--property=LoadState", "--value", unit).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "loaded", nil
}

// Enable marks the unit to start on boot.
func Enable(ctx context.Context, unit string) error {
	if unit == "" {
		return errors.New("unit name is required")
	}
	_, err := runSystemctl(ctx, "enable", unit)
	return err
}

// Disable disables the unit from starting on boot.
func Disable(ctx context.Context, unit string) error {
	if unit == "" {
		return errors.New("unit name is required")
	}
	_, err := runSystemctl(ctx, "disable", unit)
	return err
}

// EnableNow enables and starts the unit immediately.
func EnableNow(ctx context.Context, unit string) error {
	if unit == "" {
		return errors.New("unit name is required")
	}
	_, err := runSystemctl(ctx, "enable", "--now", unit)
	return err
}

// Start starts the unit. If the first attempt fails, it retries up to 3 times
// with a short delay. During bootstrap, services may fail to start on the first
// try because dependencies (etcd, TLS certs, etc.) are still initializing.
func Start(ctx context.Context, unit string) error {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
			// Reset failed state so systemd allows another start attempt
			// (avoids "Start request repeated too quickly" rate limit).
			_ = exec.CommandContext(ctx, "systemctl", "reset-failed", unit).Run()
		}
		_, err = ApplyUnitAction(ctx, unit, "start")
		if err == nil {
			return nil
		}
	}
	return err
}

// Stop stops the unit.
func Stop(ctx context.Context, unit string) error {
	_, err := ApplyUnitAction(ctx, unit, "stop")
	return err
}

// Restart restarts the unit.
func Restart(ctx context.Context, unit string) error {
	_, err := ApplyUnitAction(ctx, unit, "restart")
	return err
}

// Status returns concise unit status output.
func Status(ctx context.Context, unit string) (string, error) {
	if unit == "" {
		return "", errors.New("unit name is required")
	}
	out, err := runSystemctl(ctx, "status", unit, "--no-pager", "-n", "0")
	return out, err
}

// ReadJournalctl reads recent journal entries for a systemd unit.
func ReadJournalctl(ctx context.Context, unit string, lines int, priority string) (string, error) {
	if unit == "" {
		return "", errors.New("unit name is required")
	}
	if lines <= 0 {
		lines = 50
	}
	if lines > 200 {
		lines = 200
	}
	args := []string{"-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso"}
	if priority != "" {
		args = append(args, "-p", priority)
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// WaitActive blocks until unit becomes active or timeout expires.
func WaitActive(ctx context.Context, unit string, timeout time.Duration) error {
	if unit == "" {
		return errors.New("unit name is required")
	}
	deadline := time.Now().Add(timeout)
	for {
		active, err := IsActive(ctx, unit)
		if err != nil {
			return err
		}
		if active {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("timeout waiting for unit active")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
