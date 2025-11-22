package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

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

// makeDeterministicID builds a stable key for a (level, app, method, line, message) entry.
// Including the message avoids merging unrelated lines logged from the same site.
func makeDeterministicID(info *logpb.LogInfo, level string) string {
	return Utility.GenerateUUID(level + "|" + info.Application + "|" + info.Method + "|" + info.Line + "|" + info.Message)
}

// addToIndex ensures the log id is listed under its (level, app) index.
func (srv *server) addToIndex(idxKey, id string) {
	data, err := srv.logs.GetItem(idxKey)
	if err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil && !Utility.Contains(ids, id) {
			ids = append(ids, id)
			if enc, e := json.Marshal(ids); e == nil {
				_ = srv.logs.SetItem(idxKey, enc)
			}
		}
		return
	}
	// create new list
	ids := []string{id}
	if enc, e := json.Marshal(ids); e == nil {
		_ = srv.logs.SetItem(idxKey, enc)
	}
}

// //////////////////////////////////////////////////////////////////////////////
// Core
// //////////////////////////////////////////////////////////////////////////////
const bucketDur = time.Minute

func bucketStart(ms int64) int64 {
	return (ms / bucketDur.Milliseconds()) * bucketDur.Milliseconds()
}

// --- registry & pointers ---
// We keep the set of apps per level, and each (level,app)'s oldest/newest bucket boundaries.
func appsKey(level string) string {
	return Utility.GenerateUUID("idx_apps|" + level)
}
func oldestKey(level, app string) string {
	return Utility.GenerateUUID("idx_oldest|" + level + "|" + app)
}
func newestKey(level, app string) string {
	return Utility.GenerateUUID("idx_newest|" + level + "|" + app)
}

func (srv *server) startRetentionJanitor() {
	// Defaults if config didn’t set them
	if srv.RetentionHours <= 0 {
		srv.RetentionHours = 24 * 7
	}
	if srv.SweepEverySeconds <= 0 {
		srv.SweepEverySeconds = 300
	}

	ticker := time.NewTicker(time.Duration(srv.SweepEverySeconds) * time.Second)
	defer ticker.Stop()

	for {
		cutoff := bucketStart(time.Now().Add(-time.Duration(srv.RetentionHours) * time.Hour).UnixMilli())

		// Sweep each (level, app)
		for _, lvl := range allLevels {
			apps := srv.getAppsForLevel(lvl)
			if len(apps) == 0 {
				continue
			}
			for _, app := range apps {
				// Read current bounds
				oK := oldestKey(lvl, app)
				nK := newestKey(lvl, app)
				oldest, okOld := srv.getBoundary(oK)
				newest, okNew := srv.getBoundary(nK)
				if !okOld || !okNew {
					continue
				}
				// Nothing to sweep?
				if oldest >= cutoff {
					continue
				}
				// Don’t sweep beyond newest
				end := cutoff
				if newest < end {
					end = newest
				}

				// Sweep buckets [oldest, end)
				for b := oldest; b < end; b += bucketDur.Milliseconds() {
					// Load the time-indexed ids for this bucket
					if data, err := srv.logs.GetItem(timeIndexKey(lvl, app, b)); err == nil {
						var ids []string
						if json.Unmarshal(data, &ids) == nil {
							// Delete blobs (best effort)
							for _, id := range ids {
								_ = srv.logs.RemoveItem(id)
							}
						}
						// Remove the bucket itself
						_ = srv.logs.RemoveItem(timeIndexKey(lvl, app, b))
					}
				}

				// Advance oldest pointer to 'end'
				srv.setBoundary(oK, end)
			}
		}

		// wait for next tick
		<-ticker.C
	}
}

// addAppToLevel ensures app is listed under its level for retention sweeping.
func (srv *server) addAppToLevel(level, app string) {
	k := appsKey(level)
	data, err := srv.logs.GetItem(k)
	if err == nil {
		var apps []string
		if json.Unmarshal(data, &apps) == nil {
			if !Utility.Contains(apps, app) {
				apps = append(apps, app)
				if b, e := json.Marshal(apps); e == nil {
					_ = srv.logs.SetItem(k, b)
				}
			}
		}
		return
	}
	// create new list
	apps := []string{app}
	if b, e := json.Marshal(apps); e == nil {
		_ = srv.logs.SetItem(k, b)
	}
}

func (srv *server) getAppsForLevel(level string) []string {
	data, err := srv.logs.GetItem(appsKey(level))
	if err != nil {
		return nil
	}
	var apps []string
	if json.Unmarshal(data, &apps) != nil {
		return nil
	}
	return apps
}

func (srv *server) setBoundary(key string, ms int64) {
	_ = srv.logs.SetItem(key, []byte(strconv.FormatInt(ms, 10)))
}

func (srv *server) getBoundary(key string) (int64, bool) {
	b, err := srv.logs.GetItem(key)
	if err != nil {
		return 0, false
	}
	v, e := strconv.ParseInt(string(b), 10, 64)
	if e != nil {
		return 0, false
	}
	return v, true
}

// For convenience, the canonical set of supported levels. Keep in sync with your enums.
var allLevels = []string{"info", "debug", "error", "fatal", "trace", "warning"}

// index key for (level, app)
func indexKey(level, app string) string {
	return Utility.GenerateUUID(level + "|" + app)
}

// time index key for (level, app, bucketStartMs)
func timeIndexKey(level, app string, b int64) string {
	return Utility.GenerateUUID("idx_time|" + level + "|" + app + "|" + strconv.FormatInt(b, 10))
}

// at top of file (helpers)
func decodeMethodSegment(seg string) string {
	if seg == "" {
		return ""
	}
	// URL-decode first (%2F etc.)
	if u, err := url.PathUnescape(seg); err == nil && u != "" {
		seg = u
	}
	// Accept common "slash" escape spellings some callers might use
	seg = strings.ReplaceAll(seg, `\x2F`, "/")
	seg = strings.ReplaceAll(seg, `\x2f`, "/")
	seg = strings.ReplaceAll(seg, `\u002F`, "/")

	// Normalize to start with "/" unless wildcard
	if seg != "*" && !strings.HasPrefix(seg, "/") {
		seg = "/" + seg
	}
	return seg
}

// log persists a LogInfo, coalescing occurrences for the same (site+message),
// publishes the event to the bus, and bumps Prometheus counters.
// NOTE: This is internal; public API methods delegate here.
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

	var level string
	switch info.GetLevel() {
	case logpb.LogLevel_INFO_MESSAGE:
		level = "info"
	case logpb.LogLevel_DEBUG_MESSAGE:
		level = "debug"
	case logpb.LogLevel_ERROR_MESSAGE:
		level = "error"
	case logpb.LogLevel_FATAL_MESSAGE:
		level = "fatal"
	case logpb.LogLevel_TRACE_MESSAGE:
		level = "trace"
	case logpb.LogLevel_WARN_MESSAGE:
		level = "warning"
	default:
		level = "info"
	}

	// NEW: persist only Error/Fatal
	isPersistent := level == "error" || level == "fatal"

	if info.TimestampMs == 0 {
		info.TimestampMs = time.Now().UnixMilli()
	}

	if srv.RetentionHours > 0 {
		cutoff := time.Now().Add(-time.Duration(srv.RetentionHours) * time.Hour).UnixMilli()
		if info.TimestampMs < cutoff {
			// Drop silently; or return an error:
			// return status.Errorf(codes.FailedPrecondition, "log older than retention window")
			return nil
		}
	}

	// Stable id per (level|app|method|line|message)
	info.Id = makeDeterministicID(info, level)

	// Occurrences:
	info.Occurences = 1
	if isPersistent {
		// Only attempt coalescing if we actually persist
		if data, err := srv.logs.GetItem(info.Id); err == nil {
			prev := new(logpb.LogInfo)
			if e := protojson.Unmarshal(data, prev); e == nil {
				info.Occurences = prev.Occurences + 1
				if prev.TimestampMs > info.TimestampMs {
					info.TimestampMs = prev.TimestampMs
				}
			}
		}
	}

	// --- Persisted indices/records only for Error & Fatal ---
	if isPersistent {
		// Primary (level,app) index (coarse)
		idx := indexKey(level, info.Application)
		if data, err := srv.logs.GetItem(idx); err == nil {
			var ids []string
			if json.Unmarshal(data, &ids) == nil && !Utility.Contains(ids, info.Id) {
				ids = append(ids, info.Id)
				if b, e := json.Marshal(ids); e == nil {
					_ = srv.logs.SetItem(idx, b)
				}
			}
		} else {
			ids := []string{info.Id}
			if b, e := json.Marshal(ids); e == nil {
				_ = srv.logs.SetItem(idx, b)
			}
		}

		// Time bucket index + registry and boundaries
		bStart := bucketStart(info.TimestampMs)
		tidx := timeIndexKey(level, info.Application, bStart)
		if data, err := srv.logs.GetItem(tidx); err == nil {
			var ids []string
			if json.Unmarshal(data, &ids) == nil && !Utility.Contains(ids, info.Id) {
				ids = append(ids, info.Id)
				if b, e := json.Marshal(ids); e == nil {
					_ = srv.logs.SetItem(tidx, b)
				}
			}
		} else {
			ids := []string{info.Id}
			if b, e := json.Marshal(ids); e == nil {
				_ = srv.logs.SetItem(tidx, b)
			}
		}

		// Register app under this level (used by janitor)
		srv.addAppToLevel(level, info.Application)

		// Maintain oldest/newest bucket pointers per (level, app)
		oK := oldestKey(level, info.Application)
		nK := newestKey(level, info.Application)
		if old, ok := srv.getBoundary(oK); !ok || bStart < old {
			srv.setBoundary(oK, bStart)
		}
		if neu, ok := srv.getBoundary(nK); !ok || bStart > neu {
			srv.setBoundary(nK, bStart)
		}

		// Store the entry blob
		js, err := protojson.Marshal(info)
		if err != nil {
			return err
		}
		_ = srv.logs.SetItem(info.Id, js)
	}

	// Fan out & metrics for all levels (unchanged)
	// NOTE: we publish regardless of persistence so live UIs still see non-persistent logs.
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
//
// Required fields in rqst.Info:
//   - application, method, line
//
// Optional:
//   - message, timestamp_ms, component, fields
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

// GetLog streams up to the first 100 log entries that match a query.
//
// Query format: "/{level}/{application}/*"
// Example: "/info/dns.DnsService/*"
//
// The server sends one response containing up to 100 entries.
// GetLog streams up to the first N log entries that match a query.
//
// Query format (base):    "/{level}/{application}/*"
// Optional filters (URL): "?since=ms&until=ms&limit=N&order=asc|desc&method=Foo&component=dns&contains=sub"
// Examples:
//
//	"/info/dns.DnsService/*?since=1756650000000&limit=200&order=asc"
//	"/error/*/*?contains=timeout"
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

	// Trim leading/trailing slashes and split once.
	// We’ll allow >3 segments by re-joining the tail for the method.
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

	// defaults...
	nowMs := time.Now().UnixMilli()
	sinceMs := int64(0)
	untilMs := nowMs
	limit := 100
	orderAsc := true
	methodFilter := ""
	componentFilter := ""
	contains := ""

	// Optional method segment(s):
	// - If exactly one segment: it can be "*" or an encoded method (e.g. "%2Fsvc%2FA").
	// - If more than one segment: join the tail back to a single method string.
	if len(parts) >= 3 {
		tail := strings.Join(parts[2:], "/") // reassemble
		tail = decodeMethodSegment(tail)
		if tail != "" && tail != "*" {
			methodFilter = tail
		}
	}

	// parse filters (unchanged)...
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
			methodFilter = s // explicit query param wins
		}
		if s := v.Get("component"); s != "" {
			componentFilter = s
		}
		if s := v.Get("contains"); s != "" {
			contains = s
		}
	}
	if sinceMs > untilMs {
		sinceMs, untilMs = untilMs, sinceMs
	}

	// Helper to load a LogInfo by id
	load := func(id string) *logpb.LogInfo {
		if blob, e := srv.logs.GetItem(id); e == nil {
			var li logpb.LogInfo
			if protojson.Unmarshal(blob, &li) == nil {
				return &li
			}
		}
		return nil
	}

	// Collect candidate ids
	candidates := make(map[string]struct{})

	// Prefer time buckets if a time window is requested
	usedTimeIdx := false
	if sinceMs > 0 || untilMs < nowMs {
		for b := bucketStart(sinceMs); b <= bucketStart(untilMs); b += bucketDur.Milliseconds() {
			if data, err := srv.logs.GetItem(timeIndexKey(level, app, b)); err == nil {
				var ids []string
				if json.Unmarshal(data, &ids) == nil {
					for _, id := range ids {
						candidates[id] = struct{}{}
					}
					usedTimeIdx = true
				}
			}
		}
	}

	// Fallback: coarse (level,app) index
	if !usedTimeIdx {
		idx := indexKey(level, app)
		if data, err := srv.logs.GetItem(idx); err == nil {
			var ids []string
			if json.Unmarshal(data, &ids) == nil {
				for _, id := range ids {
					candidates[id] = struct{}{}
				}
			}
		} else {
			// no index -> nothing to send
			return stream.Send(&logpb.GetLogRsp{Infos: nil})
		}
	}

	// Load, filter, and sort
	out := make([]*logpb.LogInfo, 0, len(candidates))
	for id := range candidates {
		li := load(id)
		if li == nil {
			continue
		}
		// time window
		if (sinceMs > 0 && li.TimestampMs < sinceMs) || (untilMs > 0 && li.TimestampMs > untilMs) {
			continue
		}
		// extra filters
		if methodFilter != "" && li.Method != methodFilter {
			continue
		}
		if componentFilter != "" && li.Component != componentFilter {
			continue
		}
		if contains != "" && !strings.Contains(strings.ToLower(li.Message), strings.ToLower(contains)) {
			continue
		}
		out = append(out, li)
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

// DeleteLog removes a specific log entry and updates its (level, app) index.
//
// It expects rqst.Log to include a valid Id and Level/Application fields
// to update the correct index.
func (srv *server) DeleteLog(ctx context.Context, rqst *logpb.DeleteLogRqst) (*logpb.DeleteLogRsp, error) {
	if rqst == nil || rqst.Log == nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no log was provided")),
		)
	}

	// Remove the log blob
	if err := srv.logs.RemoveItem(rqst.Log.Id); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Remove from index
	levelStr := levelToString(rqst.Log.GetLevel())
	idx := indexKey(levelStr, rqst.Log.Application)

	if data, err := srv.logs.GetItem(idx); err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for i, id := range ids {
				if id == rqst.Log.Id {
					ids = append(ids[:i], ids[i+1:]...)
					break
				}
			}
			if enc, e := json.Marshal(ids); e == nil {
				_ = srv.logs.SetItem(idx, enc)
			}
		}
	}

	return &logpb.DeleteLogRsp{Result: true}, nil
}

// ClearAllLog removes all logs matching a query of the form "/{level}/{application}/*".
//
// It deletes each indexed entry and the index itself.
func (srv *server) ClearAllLog(ctx context.Context, rqst *logpb.ClearAllLogRqst) (*logpb.ClearAllLogRsp, error) {
	query := rqst.GetQuery()
	if query == "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no query was given")),
		)
	}

	parts := strings.Split(query, "/")

	// Expect "/{level}/{application}/*"
	// I will ignore empty parts, so "/info/dns/*" and "info/dns/*" are equivalent.
	var cleanParts []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			cleanParts = append(cleanParts, p)
		}
	}
	parts = cleanParts

	if len(parts) != 3 || parts[2] != "*" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the query must be like /{level}/{application}/*")),
		)
	}

	level, app := parts[0], parts[1]
	idx := indexKey(level, app)

	data, err := srv.logs.GetItem(idx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	for _, id := range ids {
		_ = srv.logs.RemoveItem(id) // best effort
	}
	_ = srv.logs.RemoveItem(idx) // remove the index too

	return &logpb.ClearAllLogRsp{Result: true}, nil
}
