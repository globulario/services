// detect-unmapped scans audit logs for unmapped methods (no RBAC mapping)
// and reports which methods need RBAC configuration before deny-by-default enforcement.
//
// Usage:
//   detect-unmapped [--log-file PATH] [--format table|json|csv]
//
// Environment Variables:
//   LOG_FILE   - Path to audit log file (default: /var/log/globular/audit.log)
//   LOG_FORMAT - Output format: table (default), json, csv
//
// Example:
//   # Scan default log location
//   detect-unmapped
//
//   # Scan specific log file with JSON output
//   detect-unmapped --log-file /var/log/my-audit.log --format json
//
//   # Use with journalctl
//   journalctl -u globular-* | detect-unmapped
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// AuditLogEntry represents a single audit log line
type AuditLogEntry struct {
	Level   string `json:"level"`
	Time    string `json:"time"`
	Msg     string `json:"msg"`
	Audit   string `json:"audit"` // nested JSON string
	Subject string `json:"subject"`
	Method  string `json:"method"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// AuditDecision is the nested JSON within the "audit" field
type AuditDecision struct {
	Timestamp    string `json:"timestamp"`
	Subject      string `json:"subject"`
	PrincipalType string `json:"principal_type"`
	AuthMethod   string `json:"auth_method"`
	IsLoopback   bool   `json:"is_loopback"`
	GRPCMethod   string `json:"grpc_method"`
	ResourcePath string `json:"resource_path"`
	Permission   string `json:"permission"`
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason"`
	ClusterID    string `json:"cluster_id,omitempty"`
	Bootstrap    bool   `json:"bootstrap,omitempty"`
	CallerIP     string `json:"caller_ip,omitempty"`
	CallSource   string `json:"call_source,omitempty"`
}

// MethodStats tracks statistics for an unmapped method
type MethodStats struct {
	Method     string
	CallCount  int
	UniqueUsers int
	Users      map[string]int // subject -> count
	FirstSeen  string
	LastSeen   string
}

var (
	logFile   = flag.String("log-file", "", "Path to audit log file (default: stdin or $LOG_FILE)")
	format    = flag.String("format", "table", "Output format: table, json, csv")
	showUsers = flag.Bool("show-users", false, "Show unique users for each method")
)

func main() {
	flag.Parse()

	// Determine log source
	var reader io.Reader
	if *logFile != "" {
		f, err := os.Open(*logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	} else if envFile := os.Getenv("LOG_FILE"); envFile != "" {
		f, err := os.Open(envFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file from LOG_FILE env: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	} else {
		// Read from stdin (for piping journalctl, etc.)
		reader = os.Stdin
	}

	// Parse logs
	stats, err := parseAuditLogs(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing logs: %v\n", err)
		os.Exit(1)
	}

	// Sort by call count (descending)
	methods := make([]*MethodStats, 0, len(stats))
	for _, s := range stats {
		methods = append(methods, s)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].CallCount > methods[j].CallCount
	})

	// Output
	outputFormat := strings.ToLower(*format)
	if envFmt := os.Getenv("LOG_FORMAT"); envFmt != "" {
		outputFormat = strings.ToLower(envFmt)
	}

	switch outputFormat {
	case "json":
		outputJSON(methods)
	case "csv":
		outputCSV(methods)
	default:
		outputTable(methods)
	}

	// Exit code: 0 if no unmapped methods, 1 if found
	if len(methods) > 0 {
		os.Exit(1)
	}
}

func parseAuditLogs(reader io.Reader) (map[string]*MethodStats, error) {
	stats := make(map[string]*MethodStats)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		// Try parsing as structured JSON log
		var entry AuditLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Not JSON or malformed - skip
			continue
		}

		// Only interested in authz_decision messages
		if entry.Msg != "authz_decision" {
			continue
		}

		// Only interested in unmapped method warnings
		if entry.Reason != "no_rbac_mapping_warning" {
			continue
		}

		// Parse nested audit decision
		var decision AuditDecision
		if entry.Audit != "" {
			if err := json.Unmarshal([]byte(entry.Audit), &decision); err != nil {
				// Try using top-level fields
				decision.GRPCMethod = entry.Method
				decision.Subject = entry.Subject
				decision.Allowed = entry.Allowed
				decision.Reason = entry.Reason
			}
		} else {
			// Use top-level fields
			decision.GRPCMethod = entry.Method
			decision.Subject = entry.Subject
			decision.Allowed = entry.Allowed
			decision.Reason = entry.Reason
		}

		method := decision.GRPCMethod
		if method == "" {
			continue
		}

		// Update stats
		if _, exists := stats[method]; !exists {
			stats[method] = &MethodStats{
				Method:     method,
				Users:      make(map[string]int),
				FirstSeen:  decision.Timestamp,
			}
		}

		s := stats[method]
		s.CallCount++
		s.Users[decision.Subject]++
		s.UniqueUsers = len(s.Users)
		s.LastSeen = decision.Timestamp
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading logs: %w", err)
	}

	return stats, nil
}

func outputTable(methods []*MethodStats) {
	if len(methods) == 0 {
		fmt.Println("✓ No unmapped methods found - ready for deny-by-default enforcement!")
		return
	}

	fmt.Printf("⚠  Found %d unmapped methods (need RBAC configuration)\n\n", len(methods))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METHOD\tCALLS\tUSERS\tFIRST SEEN\tLAST SEEN")
	fmt.Fprintln(w, "------\t-----\t-----\t----------\t---------")

	for _, s := range methods {
		firstSeen := truncateTimestamp(s.FirstSeen)
		lastSeen := truncateTimestamp(s.LastSeen)
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n",
			s.Method, s.CallCount, s.UniqueUsers, firstSeen, lastSeen)

		if *showUsers && len(s.Users) > 0 {
			for user, count := range s.Users {
				fmt.Fprintf(w, "  └─ %s\t%d\t\t\t\n", user, count)
			}
		}
	}
	w.Flush()

	fmt.Printf("\nTo enable deny-by-default: export GLOBULAR_DENY_UNMAPPED=1\n")
	fmt.Printf("Recommendation: Add RBAC mappings for these methods before enforcement.\n")
}

func outputJSON(methods []*MethodStats) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(methods)
}

func outputCSV(methods []*MethodStats) {
	fmt.Println("method,call_count,unique_users,first_seen,last_seen")
	for _, s := range methods {
		fmt.Printf("%s,%d,%d,%s,%s\n",
			s.Method, s.CallCount, s.UniqueUsers, s.FirstSeen, s.LastSeen)
	}
}

func truncateTimestamp(ts string) string {
	if len(ts) > 19 {
		return ts[:19] // "2026-02-10T10:05:59"
	}
	return ts
}
