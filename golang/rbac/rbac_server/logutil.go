// logutil.go: helper to route legacy prints to slog.

// @awareness namespace=globular.platform
// @awareness component=platform_rbac
// @awareness file_role=rbac_log_utilities
// @awareness risk=low
package main

import (
	"encoding/json"
	"log/slog"
	"strings"
)

// logPrintln replaces fmt.Println in legacy code with structured slog logging.
func logPrintln(args ...any) {
	var sb strings.Builder
	for i, a := range args {
		if i > 0 {
			sb.WriteString(" ")
		}
		switch v := a.(type) {
		case string:
			sb.WriteString(v)
		case error:
			sb.WriteString(v.Error())
		default:
			b, _ := json.Marshal(v)
			sb.Write(b)
		}
	}
	slog.Warn(sb.String())
}
