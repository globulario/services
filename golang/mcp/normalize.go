package main

import (
	"strings"
	"time"
)

// ── Output normalization helpers ────────────────────────────────────────────
// These convert protobuf types and raw values into AI-readable formats.

// fmtTime formats a Unix timestamp (seconds or milliseconds) into a human-readable string.
func fmtTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	// Auto-detect seconds vs milliseconds.
	if unix > 1e12 {
		return time.UnixMilli(unix).UTC().Format(time.RFC3339)
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

// fmtTimeProto formats a protobuf Timestamp to RFC3339.
func fmtTimestamp(seconds int64, nanos int32) string {
	if seconds == 0 {
		return ""
	}
	return time.Unix(seconds, int64(nanos)).UTC().Format(time.RFC3339)
}

// fmtBytes formats bytes into a human-readable size.
func fmtBytes(b uint64) string {
	switch {
	case b == 0:
		return "0 B"
	case b < 1024:
		return fmtU64(b) + " B"
	case b < 1024*1024:
		return fmtF64(float64(b)/1024) + " KB"
	case b < 1024*1024*1024:
		return fmtF64(float64(b)/(1024*1024)) + " MB"
	default:
		return fmtF64(float64(b)/(1024*1024*1024)) + " GB"
	}
}

func fmtU64(v uint64) string {
	return strings.TrimRight(strings.TrimRight(
		strings.Replace(
			func() string { s := ""; for v > 0 { s = string(rune('0'+v%10)) + s; v /= 10 }; if s == "" { s = "0" }; return s }(),
			"", "", 0), "0"), ".")
}

func fmtF64(v float64) string {
	// Simple formatting without importing strconv.
	i := int64(v)
	frac := int64((v - float64(i)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return intToStr(i) + "." + intToStr(frac)
}

func intToStr(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	s := ""
	for v > 0 {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// fmtDuration formats a duration in seconds to human-readable.
func fmtDuration(seconds float64) string {
	if seconds < 60 {
		return fmtF64(seconds) + "s"
	}
	if seconds < 3600 {
		m := int(seconds) / 60
		s := int(seconds) % 60
		return intToStr(int64(m)) + "m " + intToStr(int64(s)) + "s"
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	return intToStr(int64(h)) + "h " + intToStr(int64(m)) + "m"
}

// ago returns a human-readable "X ago" string.
func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return intToStr(int64(d.Seconds())) + "s ago"
	case d < time.Hour:
		return intToStr(int64(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return intToStr(int64(d.Hours())) + "h ago"
	default:
		return intToStr(int64(d.Hours()/24)) + "d ago"
	}
}

// getStr extracts a string from a map[string]interface{}.
func getStr(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// getInt extracts an int from a map[string]interface{}.
func getInt(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return def
	}
}

// getBool extracts a bool from a map[string]interface{}.
func getBool(args map[string]interface{}, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}
