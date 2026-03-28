package main

// tools_browser.go — Chrome DevTools Protocol (CDP) bridge.
//
// Connects to a locally-running Chrome instance via CDP (WebSocket on port 9222)
// and exposes browser diagnostics as MCP tools:
//
//   browser_console   — recent console messages and JS exceptions
//   browser_network   — recent network requests/failures
//   browser_profiler  — CPU/heap performance snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ── CDP client ──────────────────────────────────────────────────────────────

// cdpClient manages a persistent WebSocket connection to Chrome DevTools.
type cdpClient struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	nextID   int
	pending  map[int]chan json.RawMessage
	events   map[string][]cdpEvent // domain.method → recent events (ring buffer)
	maxEvents int

	// Subscribed event categories
	consoleEvents []cdpEvent
	networkEvents []cdpEvent
	exceptions    []cdpEvent
}

type cdpEvent struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Time   time.Time       `json:"time"`
}

type cdpMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newCDPClient(maxEvents int) *cdpClient {
	if maxEvents <= 0 {
		maxEvents = 200
	}
	return &cdpClient{
		pending:   make(map[int]chan json.RawMessage),
		events:    make(map[string][]cdpEvent),
		maxEvents: maxEvents,
	}
}

// connect discovers the debugger WebSocket URL and establishes a connection.
func (c *cdpClient) connect(ctx context.Context, port int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil // already connected
	}

	// Discover the WebSocket debugger URL from Chrome's /json endpoint.
	listURL := fmt.Sprintf("http://localhost:%d/json", port)
	req, _ := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Chrome not reachable on port %d — start Chrome with --remote-debugging-port=%d: %w", port, port, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		Type                 string `json:"type"`
		Title                string `json:"title"`
		URL                  string `json:"url"`
	}
	if err := json.Unmarshal(body, &targets); err != nil {
		return fmt.Errorf("parse Chrome targets: %w", err)
	}

	// Find the best page target — prefer localhost app pages, skip devtools:// URLs.
	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && !strings.HasPrefix(t.URL, "devtools://") && !strings.HasPrefix(t.URL, "chrome://") {
			wsURL = t.WebSocketDebuggerURL
			break
		}
	}
	if wsURL == "" {
		for _, t := range targets {
			if t.Type == "page" {
				wsURL = t.WebSocketDebuggerURL
				break
			}
		}
	}
	if wsURL == "" && len(targets) > 0 {
		wsURL = targets[0].WebSocketDebuggerURL
	}
	if wsURL == "" {
		return fmt.Errorf("no debuggable targets found on port %d", port)
	}

	// Connect WebSocket.
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("CDP WebSocket connect: %w", err)
	}
	c.conn = conn

	// Start reading events in background.
	go c.readLoop()

	// Enable the domains we care about.
	c.sendCommandLocked(ctx, "Runtime.enable", nil)
	c.sendCommandLocked(ctx, "Network.enable", nil)
	c.sendCommandLocked(ctx, "Console.enable", nil)

	return nil
}

func (c *cdpClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// sendCommand sends a CDP command and waits for the result.
func (c *cdpClient) sendCommand(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendCommandLocked(ctx, method, params)
}

func (c *cdpClient) sendCommandLocked(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	c.nextID++
	id := c.nextID

	var rawParams json.RawMessage
	if params != nil {
		b, _ := json.Marshal(params)
		rawParams = b
	}

	msg := cdpMessage{ID: id, Method: method, Params: rawParams}
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch

	if err := c.conn.WriteJSON(msg); err != nil {
		delete(c.pending, id)
		return nil, err
	}

	c.mu.Unlock()
	defer c.mu.Lock()

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("CDP command %s timed out", method)
	}
}

// readLoop reads CDP messages and dispatches them.
func (c *cdpClient) readLoop() {
	for {
		var msg cdpMessage
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		if err := conn.ReadJSON(&msg); err != nil {
			c.mu.Lock()
			c.conn = nil
			c.mu.Unlock()
			return
		}

		// Response to a command.
		if msg.ID > 0 {
			c.mu.Lock()
			if ch, ok := c.pending[msg.ID]; ok {
				delete(c.pending, msg.ID)
				ch <- msg.Result
			}
			c.mu.Unlock()
			continue
		}

		// Event.
		if msg.Method != "" {
			evt := cdpEvent{Method: msg.Method, Params: msg.Params, Time: time.Now()}
			c.mu.Lock()
			c.appendEvent(evt)
			c.mu.Unlock()
		}
	}
}

func (c *cdpClient) appendEvent(evt cdpEvent) {
	// Categorize into specific buffers for quick access.
	switch {
	case strings.HasPrefix(evt.Method, "Console.") || evt.Method == "Runtime.consoleAPICalled":
		c.consoleEvents = appendCapped(c.consoleEvents, evt, c.maxEvents)
	case evt.Method == "Runtime.exceptionThrown":
		c.exceptions = appendCapped(c.exceptions, evt, c.maxEvents)
	case strings.HasPrefix(evt.Method, "Network."):
		c.networkEvents = appendCapped(c.networkEvents, evt, c.maxEvents)
	}
	// Also store in the general map.
	c.events[evt.Method] = appendCapped(c.events[evt.Method], evt, c.maxEvents)
}

func appendCapped(buf []cdpEvent, evt cdpEvent, max int) []cdpEvent {
	buf = append(buf, evt)
	if len(buf) > max {
		buf = buf[len(buf)-max:]
	}
	return buf
}

// getConsole returns recent console messages + exceptions.
func (c *cdpClient) getConsole(limit int) []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	var results []map[string]interface{}

	// Console API calls (log, warn, error)
	for _, evt := range c.tail(c.consoleEvents, limit) {
		var params struct {
			Type string `json:"type"`
			Args []struct {
				Type  string      `json:"type"`
				Value interface{} `json:"value"`
				Desc  string      `json:"description"`
			} `json:"args"`
		}
		json.Unmarshal(evt.Params, &params)

		parts := make([]string, 0, len(params.Args))
		for _, a := range params.Args {
			if a.Desc != "" {
				parts = append(parts, a.Desc)
			} else if a.Value != nil {
				parts = append(parts, fmt.Sprintf("%v", a.Value))
			}
		}
		results = append(results, map[string]interface{}{
			"type":    params.Type,
			"message": strings.Join(parts, " "),
			"time":    evt.Time.Format(time.RFC3339),
		})
	}

	// Exceptions
	for _, evt := range c.tail(c.exceptions, limit) {
		var params struct {
			ExceptionDetails struct {
				Text      string `json:"text"`
				Exception struct {
					Description string `json:"description"`
				} `json:"exception"`
				StackTrace struct {
					CallFrames []struct {
						FunctionName string `json:"functionName"`
						URL          string `json:"url"`
						LineNumber   int    `json:"lineNumber"`
						ColumnNumber int    `json:"columnNumber"`
					} `json:"callFrames"`
				} `json:"stackTrace"`
			} `json:"exceptionDetails"`
		}
		json.Unmarshal(evt.Params, &params)
		d := params.ExceptionDetails

		stack := make([]string, 0)
		for _, f := range d.StackTrace.CallFrames {
			stack = append(stack, fmt.Sprintf("  at %s (%s:%d:%d)", f.FunctionName, f.URL, f.LineNumber, f.ColumnNumber))
		}

		results = append(results, map[string]interface{}{
			"type":    "exception",
			"message": d.Text + ": " + d.Exception.Description,
			"stack":   strings.Join(stack, "\n"),
			"time":    evt.Time.Format(time.RFC3339),
		})
	}

	return results
}

// getNetwork returns recent network requests with status and timing.
func (c *cdpClient) getNetwork(limit int, failedOnly bool) []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build a map of requestId → info from all network events.
	type reqInfo struct {
		URL        string
		Method     string
		Status     int
		StatusText string
		Type       string
		Failed     bool
		Error      string
		Timing     float64 // ms
		Time       time.Time
	}
	requests := make(map[string]*reqInfo)

	for _, evt := range c.networkEvents {
		switch evt.Method {
		case "Network.requestWillBeSent":
			var p struct {
				RequestID string `json:"requestId"`
				Request   struct {
					URL    string `json:"url"`
					Method string `json:"method"`
				} `json:"request"`
				Type string `json:"type"`
			}
			json.Unmarshal(evt.Params, &p)
			requests[p.RequestID] = &reqInfo{
				URL: p.Request.URL, Method: p.Request.Method,
				Type: p.Type, Time: evt.Time,
			}

		case "Network.responseReceived":
			var p struct {
				RequestID string `json:"requestId"`
				Response  struct {
					Status     int     `json:"status"`
					StatusText string  `json:"statusText"`
					Timing     struct {
						ReceiveHeadersEnd float64 `json:"receiveHeadersEnd"`
					} `json:"timing"`
				} `json:"response"`
			}
			json.Unmarshal(evt.Params, &p)
			if r, ok := requests[p.RequestID]; ok {
				r.Status = p.Response.Status
				r.StatusText = p.Response.StatusText
				r.Timing = p.Response.Timing.ReceiveHeadersEnd
				if p.Response.Status >= 400 {
					r.Failed = true
				}
			}

		case "Network.loadingFailed":
			var p struct {
				RequestID    string `json:"requestId"`
				ErrorText    string `json:"errorText"`
				Canceled     bool   `json:"canceled"`
				BlockedReason string `json:"blockedReason"`
			}
			json.Unmarshal(evt.Params, &p)
			if r, ok := requests[p.RequestID]; ok {
				r.Failed = true
				r.Error = p.ErrorText
				if p.BlockedReason != "" {
					r.Error += " (blocked: " + p.BlockedReason + ")"
				}
			}
		}
	}

	var results []map[string]interface{}
	for _, r := range requests {
		if failedOnly && !r.Failed {
			continue
		}
		entry := map[string]interface{}{
			"url":    r.URL,
			"method": r.Method,
			"status": r.Status,
			"type":   r.Type,
			"time":   r.Time.Format(time.RFC3339),
		}
		if r.StatusText != "" {
			entry["status_text"] = r.StatusText
		}
		if r.Timing > 0 {
			entry["timing_ms"] = r.Timing
		}
		if r.Failed {
			entry["failed"] = true
		}
		if r.Error != "" {
			entry["error"] = r.Error
		}
		results = append(results, entry)
	}

	// Limit results (most recent first).
	if len(results) > limit {
		results = results[len(results)-limit:]
	}
	return results
}

// getPerformance returns current performance metrics.
func (c *cdpClient) getPerformance(ctx context.Context) (map[string]interface{}, error) {
	// Enable Performance domain.
	c.sendCommand(ctx, "Performance.enable", nil)

	result, err := c.sendCommand(ctx, "Performance.getMetrics", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Metrics []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"metrics"`
	}
	json.Unmarshal(result, &resp)

	metrics := make(map[string]interface{})
	for _, m := range resp.Metrics {
		metrics[m.Name] = m.Value
	}
	return metrics, nil
}

func (c *cdpClient) tail(events []cdpEvent, n int) []cdpEvent {
	if len(events) <= n {
		return events
	}
	return events[len(events)-n:]
}

// ── Singleton CDP client ────────────────────────────────────────────────────

var (
	globalCDP     *cdpClient
	globalCDPOnce sync.Once
)

func getCDP() *cdpClient {
	globalCDPOnce.Do(func() {
		globalCDP = newCDPClient(500)
	})
	return globalCDP
}

// ── MCP tool registration ───────────────────────────────────────────────────

// evaluate runs JavaScript in the browser and returns the result.
func (c *cdpClient) evaluate(ctx context.Context, expression string) (interface{}, error) {
	result, err := c.sendCommand(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Type  string      `json:"type"`
			Value interface{} `json:"value"`
			Desc  string      `json:"description"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	json.Unmarshal(result, &resp)
	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("JS error: %s", resp.ExceptionDetails.Text)
	}
	if resp.Result.Value != nil {
		return resp.Result.Value, nil
	}
	return resp.Result.Desc, nil
}

func registerBrowserTools(s *server) {

	s.register(toolDef{
		Name: "browser_console",
		Description: `Fetch recent browser console messages and JavaScript exceptions from a locally-running Chrome instance.
Requires Chrome started with --remote-debugging-port=9222.
Returns console.log/warn/error messages and uncaught exceptions with stack traces.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"port":  {Type: "integer", Description: "Chrome debugging port (default: 9222)"},
				"limit": {Type: "integer", Description: "Max messages to return (default: 50)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		port := getInt(args, "port", 9222)
		limit := getInt(args, "limit", 50)

		cdp := getCDP()
		if err := cdp.connect(ctx, port); err != nil {
			return nil, err
		}

		messages := cdp.getConsole(limit)
		return map[string]interface{}{
			"count":    len(messages),
			"messages": messages,
		}, nil
	})

	s.register(toolDef{
		Name: "browser_network",
		Description: `Fetch recent network requests from a locally-running Chrome instance.
Shows URLs, HTTP methods, status codes, timing, and error details.
Use failed_only=true to see only failed requests (4xx, 5xx, CORS, connection errors).
Requires Chrome started with --remote-debugging-port=9222.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"port":        {Type: "integer", Description: "Chrome debugging port (default: 9222)"},
				"limit":       {Type: "integer", Description: "Max requests to return (default: 50)"},
				"failed_only": {Type: "boolean", Description: "Only show failed requests (default: false)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		port := getInt(args, "port", 9222)
		limit := getInt(args, "limit", 50)
		failedOnly := getBool(args, "failed_only", false)

		cdp := getCDP()
		if err := cdp.connect(ctx, port); err != nil {
			return nil, err
		}

		requests := cdp.getNetwork(limit, failedOnly)
		return map[string]interface{}{
			"count":    len(requests),
			"requests": requests,
		}, nil
	})

	s.register(toolDef{
		Name: "browser_profiler",
		Description: `Get performance metrics from a locally-running Chrome instance.
Returns JS heap size, DOM node count, layout count, script duration, and other Chrome performance counters.
Requires Chrome started with --remote-debugging-port=9222.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"port": {Type: "integer", Description: "Chrome debugging port (default: 9222)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		port := getInt(args, "port", 9222)

		cdp := getCDP()
		if err := cdp.connect(ctx, port); err != nil {
			return nil, err
		}

		metrics, err := cdp.getPerformance(ctx)
		if err != nil {
			return nil, fmt.Errorf("get performance metrics: %w", err)
		}

		return map[string]interface{}{
			"metrics": metrics,
		}, nil
	})

	s.register(toolDef{
		Name: "browser_evaluate",
		Description: `Execute JavaScript in the browser page context and return the result.
Can read DOM state, call functions, inspect variables, or write to the console.
Requires Chrome started with --remote-debugging-port=9222.

Examples:
  "document.title" → page title
  "document.querySelectorAll('.error').length" → count error elements
  "window.location.hash" → current route
  "console.log('hello from Claude')" → write to browser console`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"expression": {Type: "string", Description: "JavaScript expression to evaluate"},
				"port":       {Type: "integer", Description: "Chrome debugging port (default: 9222)"},
			},
			Required: []string{"expression"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		expr := getStr(args, "expression")
		port := getInt(args, "port", 9222)

		if expr == "" {
			return nil, fmt.Errorf("expression is required")
		}

		cdp := getCDP()
		if err := cdp.connect(ctx, port); err != nil {
			return nil, err
		}

		result, err := cdp.evaluate(ctx, expr)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"result": result,
		}, nil
	})
}
