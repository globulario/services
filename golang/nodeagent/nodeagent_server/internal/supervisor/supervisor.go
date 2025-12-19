package supervisor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
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
