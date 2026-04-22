package main

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// Rate-limit "store write failed" warnings to once per 30s to prevent stderr
// storms when ScyllaDB is down and the log service is receiving high RPC volume.
var lastStoreWriteWarnAt atomic.Int64 // unix seconds of last warn emission

////////////////////////////////////////////////////////////////////////////////
// Internal helpers
////////////////////////////////////////////////////////////////////////////////

func levelToString(lvl logpb.LogLevel) string {
	switch lvl {
	case logpb.LogLevel_INFO_MESSAGE:
		return "info"
	case logpb.LogLevel_DEBUG_MESSAGE:
		return "debug"
	case logpb.LogLevel_ERROR_MESSAGE:
		return "error"
	case logpb.LogLevel_FATAL_MESSAGE:
		return "fatal"
	case logpb.LogLevel_TRACE_MESSAGE:
		return "trace"
	case logpb.LogLevel_WARN_MESSAGE:
		return "warning"
	default:
		return "info"
	}
}

func stringToLevel(s string) logpb.LogLevel {
	switch s {
	case "error":
		return logpb.LogLevel_ERROR_MESSAGE
	case "fatal":
		return logpb.LogLevel_FATAL_MESSAGE
	case "warning":
		return logpb.LogLevel_WARN_MESSAGE
	case "debug":
		return logpb.LogLevel_DEBUG_MESSAGE
	case "trace":
		return logpb.LogLevel_TRACE_MESSAGE
	default:
		return logpb.LogLevel_INFO_MESSAGE
	}
}

// makeDeterministicID builds a stable key for a (level, app, method, line) entry.
// The message is intentionally excluded so that errors from the same code location
// (differing only in dynamic parameters) coalesce into a single entry with an
// incremented occurrence count.  The most recent message is preserved on write.
func makeDeterministicID(info *logpb.LogInfo, level string) string {
	return Utility.GenerateUUID(level + "|" + info.Application + "|" + info.Method + "|" + info.Line)
}

// dayBucket converts a unix-millis timestamp to a day index (days since epoch).
func dayBucket(ms int64) int {
	return int(ms / 86400000)
}

// decodeMethodSegment decodes URL-encoded method path segments.
func decodeMethodSegment(seg string) string {
	if seg == "" {
		return ""
	}
	if u, err := url.PathUnescape(seg); err == nil && u != "" {
		seg = u
	}
	seg = strings.ReplaceAll(seg, `\x2F`, "/")
	seg = strings.ReplaceAll(seg, `\x2f`, "/")
	seg = strings.ReplaceAll(seg, `\u002F`, "/")

	if seg != "*" && !strings.HasPrefix(seg, "/") {
		seg = "/" + seg
	}
	return seg
}

var allLevels = []string{"info", "debug", "error", "fatal", "trace", "warning"}

////////////////////////////////////////////////////////////////////////////////
// Key helpers for the Store-based layout
//
// Entries store key:  {level}:{application}:{day_bucket}:{id}
// Registry store key: {level}:{application}
////////////////////////////////////////////////////////////////////////////////

func entryKey(level, app string, bucket int, id string) string {
	return level + ":" + app + ":" + strconv.Itoa(bucket) + ":" + id
}

func registryKey(level, app string) string {
	return level + ":" + app
}

// parseEntryKey splits a composite entry key back into its parts.
func parseEntryKey(key string) (level, app string, bucket int, id string, ok bool) {
	// Format: {level}:{application}:{day_bucket}:{id}
	// We split from the left: level is first segment, app is second,
	// bucket is third, id is everything after the third colon.
	i1 := strings.IndexByte(key, ':')
	if i1 < 0 {
		return
	}
	rest := key[i1+1:]
	i2 := strings.IndexByte(rest, ':')
	if i2 < 0 {
		return
	}
	rest2 := rest[i2+1:]
	i3 := strings.IndexByte(rest2, ':')
	if i3 < 0 {
		return
	}

	level = key[:i1]
	app = rest[:i2]
	b, err := strconv.Atoi(rest2[:i3])
	if err != nil {
		return
	}
	bucket = b
	id = rest2[i3+1:]
	ok = true
	return
}

// parseRegistryKey splits a registry key into level and application.
func parseRegistryKey(key string) (level, app string, ok bool) {
	i := strings.IndexByte(key, ':')
	if i < 0 {
		return
	}
	level = key[:i]
	app = key[i+1:]
	ok = true
	return
}

////////////////////////////////////////////////////////////////////////////////
// Core: log()
////////////////////////////////////////////////////////////////////////////////

// log persists a LogInfo (error/fatal only), coalescing occurrences,
// publishes the event to the bus, and bumps Prometheus counters.
func (srv *server) log(info *logpb.LogInfo) error {
	if info == nil {
		return errors.New("no log info was given")
	}
	if len(info.Application) == 0 {
		return errors.New("no application name was given")
	}
	if len(info.Method) == 0 {
		return errors.New("no method name was given")
	}
	if len(info.Line) == 0 {
		return errors.New("no line number was given")
	}

	level := levelToString(info.GetLevel())
	isPersistent := level == "error" || level == "fatal"

	if info.TimestampMs == 0 {
		info.TimestampMs = time.Now().UnixMilli()
	}

	if srv.RetentionHours > 0 {
		cutoff := time.Now().Add(-time.Duration(srv.RetentionHours) * time.Hour).UnixMilli()
		if info.TimestampMs < cutoff {
			return nil
		}
	}

	info.Id = makeDeterministicID(info, level)

	info.Occurences = 1
	if isPersistent {
		entries, err := srv.getStore("log_entries")
		if err != nil {
			logger.Warn("store open failed", "store", "log_entries", "err", err)
		} else {
			bucket := dayBucket(info.TimestampMs)
			key := entryKey(level, info.Application, bucket, info.Id)

			// Coalesce: read existing entry
			if raw, err := entries.GetItem(key); err == nil && len(raw) > 0 {
				var prev logpb.LogInfo
				if err := protojson.Unmarshal(raw, &prev); err == nil {
					info.Occurences = prev.Occurences + 1
					if prev.TimestampMs > info.TimestampMs {
						info.TimestampMs = prev.TimestampMs
					}
				}
			}

			// Write entry
			if data, err := protojson.Marshal(info); err == nil {
				if err := entries.SetItem(key, data); err != nil {
					now := time.Now().Unix()
					if prev := lastStoreWriteWarnAt.Swap(now); now-prev > 30 {
						logger.Warn("store write failed (suppressing repeats for 30s)", "err", err)
					}
				}
			}

			// Update registry
			if reg, err := srv.getStore("log_registry"); err == nil {
				rk := registryKey(level, info.Application)
				_ = reg.SetItem(rk, []byte("1"))
			}
		}
	}

	// Publish event for all levels (live tail)
	js, _ := protojson.Marshal(info)
	srv.publish("new_log_evt", js)
	srv.logCount.WithLabelValues(level, info.Application, info.Method).Inc()
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// API
////////////////////////////////////////////////////////////////////////////////

// Log receives a log entry, validates the caller token, persists/coalesces it,
// publishes a `new_log_evt` with the full payload, and returns success.
func (srv *server) Log(ctx context.Context, rqst *logpb.LogRqst) (*logpb.LogRsp, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = security.ValidateToken(token); err != nil {
		return nil, err
	}

	if err := srv.log(rqst.Info); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	return &logpb.LogRsp{Result: true}, nil
}

// GetLog streams log entries that match a query.
//
// Query format (base):    "/{level}/{application}/*"
// Optional filters (URL): "?since=ms&until=ms&limit=N&order=asc|desc&method=Foo&component=dns&contains=sub&node=hostname"
func (srv *server) GetLog(rqst *logpb.GetLogRqst, stream logpb.LogService_GetLogServer) error {

	q := rqst.Query
	if strings.TrimSpace(q) == "" {
		return errors.New("no query was given")
	}

	var rawPath, rawQuery string
	if i := strings.IndexByte(q, '?'); i >= 0 {
		rawPath, rawQuery = q[:i], q[i+1:]
	} else {
		rawPath = q
	}

	raw := strings.Trim(rawPath, "/")
	if raw == "" {
		return errors.New("the query must be like /{level}/{application}[/[*|method]]")
	}
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return errors.New("the query must be like /{level}/{application}[/[*|method]]")
	}

	level := strings.ToLower(parts[0])
	app := parts[1]

	nowMs := time.Now().UnixMilli()
	sinceMs := int64(0)
	untilMs := nowMs
	limit := 100
	orderAsc := true
	methodFilter := ""
	componentFilter := ""
	contains := ""
	nodeFilter := ""

	if len(parts) >= 3 {
		tail := strings.Join(parts[2:], "/")
		tail = decodeMethodSegment(tail)
		if tail != "" && tail != "*" {
			methodFilter = tail
		}
	}

	if rawQuery != "" {
		v, _ := url.ParseQuery(rawQuery)
		if s := v.Get("since"); s != "" {
			if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
				sinceMs = ms
			}
		}
		if s := v.Get("until"); s != "" {
			if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
				untilMs = ms
			}
		}
		if s := v.Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		if s := strings.ToLower(v.Get("order")); s == "desc" {
			orderAsc = false
		}
		if s := v.Get("method"); s != "" {
			methodFilter = s
		}
		if s := v.Get("component"); s != "" {
			componentFilter = s
		}
		if s := v.Get("contains"); s != "" {
			contains = s
		}
		if s := v.Get("node"); s != "" {
			nodeFilter = s
		}
	}
	if sinceMs > untilMs {
		sinceMs, untilMs = untilMs, sinceMs
	}

	entries, err := srv.getStore("log_entries")
	if err != nil {
		return stream.Send(&logpb.GetLogRsp{Infos: nil})
	}

	// Expand wildcard level/app into concrete (level, app) pairs via registry.
	type pair struct{ lvl, ap string }
	var pairs []pair

	if level == "*" || app == "*" {
		reg, err := srv.getStore("log_registry")
		if err != nil {
			return stream.Send(&logpb.GetLogRsp{Infos: nil})
		}
		regKeys, err := reg.GetAllKeys()
		if err != nil {
			return stream.Send(&logpb.GetLogRsp{Infos: nil})
		}
		for _, rk := range regKeys {
			rl, ra, ok := parseRegistryKey(rk)
			if !ok {
				continue
			}
			matchLevel := level == "*" || rl == level
			matchApp := app == "*" || ra == app
			if matchLevel && matchApp {
				pairs = append(pairs, pair{rl, ra})
			}
		}
	} else {
		pairs = []pair{{level, app}}
	}

	if len(pairs) == 0 {
		return stream.Send(&logpb.GetLogRsp{Infos: nil})
	}

	// Compute day_bucket range
	retentionMs := int64(srv.retentionDays()) * 86400000
	if sinceMs == 0 {
		sinceMs = nowMs - retentionMs
	}
	startBucket := dayBucket(sinceMs)
	endBucket := dayBucket(untilMs)

	// Build a set of wanted (level:app:) prefixes for fast filtering
	wantedPrefixes := make(map[string]string) // prefix -> level (for enum conversion)
	for _, p := range pairs {
		wantedPrefixes[p.lvl+":"+p.ap+":"] = p.lvl
	}

	// Scan all keys and filter
	allKeys, err := entries.GetAllKeys()
	if err != nil {
		return stream.Send(&logpb.GetLogRsp{Infos: nil})
	}

	seen := make(map[string]struct{})
	var out []*logpb.LogInfo

	for _, k := range allKeys {
		kLevel, kApp, kBucket, _, ok := parseEntryKey(k)
		if !ok {
			continue
		}
		prefix := kLevel + ":" + kApp + ":"
		if _, wanted := wantedPrefixes[prefix]; !wanted {
			continue
		}
		if kBucket < startBucket || kBucket > endBucket {
			continue
		}

		raw, err := entries.GetItem(k)
		if err != nil || len(raw) == 0 {
			continue
		}

		var info logpb.LogInfo
		if err := protojson.Unmarshal(raw, &info); err != nil {
			continue
		}

		if _, dup := seen[info.Id]; dup {
			continue
		}
		seen[info.Id] = struct{}{}

		// Time window filter
		if info.TimestampMs < sinceMs || info.TimestampMs > untilMs {
			continue
		}
		// Post-fetch filters
		if methodFilter != "" && info.Method != methodFilter {
			continue
		}
		if componentFilter != "" && info.Component != componentFilter {
			continue
		}
		if contains != "" && !strings.Contains(strings.ToLower(info.Message), strings.ToLower(contains)) {
			continue
		}
		if nodeFilter != "" && info.NodeId != nodeFilter {
			continue
		}

		// Ensure level enum is set correctly (protojson may decode it, but be safe)
		info.Level = stringToLevel(kLevel)

		out = append(out, &info)
	}

	sort.Slice(out, func(i, j int) bool {
		if orderAsc {
			return out[i].TimestampMs < out[j].TimestampMs
		}
		return out[i].TimestampMs > out[j].TimestampMs
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return stream.Send(&logpb.GetLogRsp{Infos: out})
}

// DeleteLog removes a specific log entry.
func (srv *server) DeleteLog(ctx context.Context, rqst *logpb.DeleteLogRqst) (*logpb.DeleteLogRsp, error) {
	if rqst == nil || rqst.Log == nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no log was provided")),
		)
	}

	entries, err := srv.getStore("log_entries")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store unavailable: %s", err.Error())
	}

	levelStr := levelToString(rqst.Log.GetLevel())

	if rqst.Log.TimestampMs > 0 {
		// Direct delete by known key
		bucket := dayBucket(rqst.Log.TimestampMs)
		key := entryKey(levelStr, rqst.Log.Application, bucket, rqst.Log.Id)
		if err := entries.RemoveItem(key); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", err.Error())
		}
	} else {
		// Scan retention window to find the entry
		nowMs := time.Now().UnixMilli()
		retentionMs := int64(srv.retentionDays()) * 86400000
		startBucket := dayBucket(nowMs - retentionMs)
		endBucket := dayBucket(nowMs)
		for b := startBucket; b <= endBucket; b++ {
			key := entryKey(levelStr, rqst.Log.Application, b, rqst.Log.Id)
			_ = entries.RemoveItem(key)
		}
	}

	return &logpb.DeleteLogRsp{Result: true}, nil
}

// ClearAllLog removes all logs matching a query of the form "/{level}/{application}/*".
func (srv *server) ClearAllLog(ctx context.Context, rqst *logpb.ClearAllLogRqst) (*logpb.ClearAllLogRsp, error) {
	query := rqst.GetQuery()
	if query == "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no query was given")),
		)
	}

	var cleanParts []string
	for _, p := range strings.Split(query, "/") {
		if p = strings.TrimSpace(p); p != "" {
			cleanParts = append(cleanParts, p)
		}
	}

	if len(cleanParts) != 3 || cleanParts[2] != "*" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the query must be like /{level}/{application}/*")),
		)
	}

	entries, err := srv.getStore("log_entries")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store unavailable: %s", err.Error())
	}

	level, app := cleanParts[0], cleanParts[1]
	prefix := level + ":" + app + ":"

	allKeys, err := entries.GetAllKeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list keys: %s", err.Error())
	}

	for _, k := range allKeys {
		if strings.HasPrefix(k, prefix) {
			_ = entries.RemoveItem(k)
		}
	}

	// Remove from registry
	if reg, err := srv.getStore("log_registry"); err == nil {
		rk := registryKey(level, app)
		_ = reg.RemoveItem(rk)
	}

	return &logpb.ClearAllLogRsp{Result: true}, nil
}
