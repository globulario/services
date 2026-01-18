package log_client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/testutil"
)

func randStr(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf))
}

func newClient(t *testing.T) *Log_Client {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	c, err := NewLogService_Client(addr, "log.LogService")
	if err != nil {
		t.Fatalf("NewLogService_Client: %v", err)
	}
	return c
}

func mustLog(t *testing.T, c *Log_Client, app, method, msg string, lvl logpb.LogLevel) {
	t.Helper()
	ctx := c.GetCtx()

	// Get local token
	mac, _ := config.GetMacAddress()
	token, _ := security.GetLocalToken(mac)

	if err := c.LogCtx(ctx, app, "golang", method, lvl, msg, "L42", "Test", token); err != nil {
		t.Fatalf("Log: %v", err)
	}
}

func getLog(t *testing.T, c *Log_Client, q string) []*logpb.LogInfo {
	t.Helper()
	ctx := c.GetCtx()
	infos, err := c.GetLogCtx(ctx, q)
	if err != nil {
		t.Fatalf("GetLog %q: %v", q, err)
	}
	return infos
}

// --- tests -------------------------------------------------------------------

// TestLogAndGetBasic appends a few entries and fetches them by base query.
func TestLogAndGetBasic(t *testing.T) {

	c := newClient(t)
	app := randStr("app_basic")

	mustLog(t, c, app, "/svc/Op", "hello 1", logpb.LogLevel_INFO_MESSAGE)
	mustLog(t, c, app, "/svc/Op", "hello 2", logpb.LogLevel_INFO_MESSAGE)
	mustLog(t, c, app, "/svc/Op", "hello 3", logpb.LogLevel_INFO_MESSAGE)

	infos := getLog(t, c, "/info/"+app+"/*?limit=10&order=asc")
	if len(infos) == 0 {
		t.Fatalf("expected some infos for %s", app)
	}
	for _, li := range infos {
		if li.Application != app {
			t.Fatalf("unexpected app: %s", li.Application)
		}
		if li.Method != "/svc/Op" {
			t.Fatalf("unexpected method: %s", li.Method)
		}
	}
}

// TestFilters_TimeAndContains verifies since/until + contains work.
func TestFilters_TimeAndContains(t *testing.T) {
	c := newClient(t)
	app := randStr("app_filters")

	mustLog(t, c, app, "/svc/A", "alpha needle", logpb.LogLevel_INFO_MESSAGE)
	t0 := time.Now().UnixMilli()
	mustLog(t, c, app, "/svc/A", "alpha needle", logpb.LogLevel_INFO_MESSAGE)
	time.Sleep(10 * time.Millisecond)
	mustLog(t, c, app, "/svc/A", "beta haystack", logpb.LogLevel_INFO_MESSAGE)

	q := fmt.Sprintf("/info/%s/*?since=%d&contains=needle", app, t0)
	infos := getLog(t, c, q)
	if len(infos) < 1 {
		t.Fatalf("expected at least one match for contains filter")
	}
	for _, li := range infos {
		if li.Application != app || li.Method != "/svc/A" || li.Message == "" {
			t.Fatalf("unexpected fields in filtered result: %+v", li)
		}
	}
}

// TestMethodExact checks method filtering via path segment and via query param.
func TestMethodExact(t *testing.T) {
	c := newClient(t)
	app := randStr("app_method")

	mustLog(t, c, app, "/svc/A", "one", logpb.LogLevel_INFO_MESSAGE)
	mustLog(t, c, app, "/svc/B", "two", logpb.LogLevel_INFO_MESSAGE)

	// Using 3rd segment
	infos := getLog(t, c, "/info/"+app+"/\x2Fsvc\x2FA") // '/svc/A' literal in path
	if len(infos) == 0 {
		t.Fatalf("expected entries for exact method path filter")
	}
	for _, li := range infos {
		if li.Method != "/svc/A" {
			t.Fatalf("got method %s; want /svc/A", li.Method)
		}
	}

	// Using method= query param
	infos2 := getLog(t, c, "/info/"+app+"/*?method=/svc/B")
	if len(infos2) == 0 {
		t.Fatalf("expected entries for method query filter")
	}
	for _, li := range infos2 {
		if li.Method != "/svc/B" {
			t.Fatalf("got method %s; want /svc/B", li.Method)
		}
	}
}

// TestDeleteLog removes a single entry and ensures it no longer appears.
func TestDeleteLog(t *testing.T) {
	c := newClient(t)
	app := randStr("app_delete")

	// Get local token
	mac, _ := config.GetMacAddress()
	token, _ := security.GetLocalToken(mac)

	mustLog(t, c, app, "/svc/Del", "to-delete", logpb.LogLevel_INFO_MESSAGE)
	infos := getLog(t, c, "/info/"+app+"/*?limit=5")
	if len(infos) == 0 {
		t.Fatalf("no entries to delete")
	}
	if err := c.DeleteLog(infos[0], token); err != nil {
		t.Fatalf("DeleteLog: %v", err)
	}
	// Fetch again and ensure the deleted id is gone
	infos2 := getLog(t, c, "/info/"+app+"/*?limit=5")
	for _, li := range infos2 {
		if li.Id == infos[0].Id {
			t.Fatalf("deleted id still present in results")
		}
	}
}

// TestClearAllLog wipes all entries for an app/level.
func TestClearAllLog(t *testing.T) {
	c := newClient(t)
	app := randStr("app_clear")

	for i := range 3 {
		mustLog(t, c, app, "/svc/Clear", fmt.Sprintf("msg-%d", i), logpb.LogLevel_INFO_MESSAGE)
	}
	// Sanity precondition
	pre := getLog(t, c, "/info/"+app+"/*")
	if len(pre) == 0 {
		t.Fatalf("expected precondition entries before clear")
	}

	// Get local token
	mac, _ := config.GetMacAddress()
	token, _ := security.GetLocalToken(mac)

	if err := c.ClearLog("/info/"+app+"/*", token); err != nil {
		t.Fatalf("ClearLog: %v", err)
	}
	post := getLog(t, c, "/info/"+app+"/*")
	if len(post) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(post))
	}
}

// TestBadQuery ensures the server returns an error for invalid queries.
func TestBadQuery(t *testing.T) {
	c := newClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.GetLogCtx(ctx, "badquery"); err == nil {
		t.Fatalf("expected error for malformed query")
	}
}
