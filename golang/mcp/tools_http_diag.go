package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"
)

func registerHTTPDiagTools(s *server) {

	s.register(toolDef{
		Name: "http_diagnose",
		Description: `Diagnose HTTP/HTTPS request latency from inside the server using net/http/httptrace.

Measures every phase of an HTTP request: DNS lookup, TCP connect, TLS handshake,
time-to-first-byte (TTFB), and total time. Useful for diagnosing:
- Gateway file serving latency (Range requests, moov atom fetch)
- Envoy proxy overhead (compare localhost:8443 vs localhost:443)
- TLS/HTTP2 negotiation issues
- Slow disk I/O (compare TTFB for beginning vs end of large files)

Examples:
  # Test if Range requests work for a video file
  http_diagnose(url="https://localhost:8443/mnt/disk/video.mp4", range_start=0, range_end=1024)

  # Check moov atom position (compare TTFB for start vs end of file)
  http_diagnose(url="https://localhost:8443/mnt/disk/video.mp4", range_start=-1024)

  # Compare direct Gateway vs through Envoy
  http_diagnose(url="https://localhost:8443/path/file.mp4", range_start=0, range_end=1024)
  http_diagnose(url="https://localhost:443/path/file.mp4", range_start=0, range_end=1024)

  # HEAD request to check headers without downloading
  http_diagnose(url="https://localhost:8443/path/file.mp4", method="HEAD")`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"url":         {Type: "string", Description: "Full URL to request (e.g. 'https://localhost:8443/path/to/file.mp4')"},
				"method":      {Type: "string", Description: "HTTP method (default: GET)", Enum: []string{"GET", "HEAD"}},
				"token":       {Type: "string", Description: "Auth token — appended as ?token= query param if provided"},
				"range_start": {Type: "number", Description: "Range start byte offset. Negative value means from end of file (e.g. -1024 = last 1KB)"},
				"range_end":   {Type: "number", Description: "Range end byte offset (optional, 0 = open-ended range)"},
				"max_body":    {Type: "number", Description: "Max response bytes to consume (default 4096, prevents downloading entire file)"},
				"timeout_sec": {Type: "number", Description: "Request timeout in seconds (default 30)"},
				"skip_verify": {Type: "boolean", Description: "Skip TLS certificate verification (default true for localhost)"},
			},
			Required: []string{"url"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawURL := getStr(args, "url")
		if rawURL == "" {
			return nil, fmt.Errorf("url is required")
		}

		method := strings.ToUpper(getStr(args, "method"))
		if method == "" {
			method = "GET"
		}
		if method != "GET" && method != "HEAD" {
			return nil, fmt.Errorf("method must be GET or HEAD")
		}

		// Append token as query param if provided
		if token := getStr(args, "token"); token != "" {
			u, err := url.Parse(rawURL)
			if err != nil {
				return nil, fmt.Errorf("invalid url: %w", err)
			}
			q := u.Query()
			q.Set("token", token)
			u.RawQuery = q.Encode()
			rawURL = u.String()
		}

		rangeStart := int64(getInt(args, "range_start", 0))
		rangeEnd := int64(getInt(args, "range_end", 0))
		maxBody := int64(getInt(args, "max_body", 4096))
		timeoutSec := getInt(args, "timeout_sec", 30)
		skipVerify := getBool(args, "skip_verify", true)

		// Build HTTP client with optional TLS skip
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify, // #nosec G402 — diagnostic tool, user-controlled
			},
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeoutSec) * time.Second,
		}

		// Trace timings
		var dnsStart, connStart, tlsStart, reqStart time.Time
		var dnsDur, connDur, tlsDur, ttfb time.Duration

		trace := &httptrace.ClientTrace{
			DNSStart:             func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone:              func(_ httptrace.DNSDoneInfo) { dnsDur = time.Since(dnsStart) },
			ConnectStart:         func(_, _ string) { connStart = time.Now() },
			ConnectDone:          func(_, _ string, _ error) { connDur = time.Since(connStart) },
			TLSHandshakeStart:    func() { tlsStart = time.Now() },
			TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsDur = time.Since(tlsStart) },
			GotFirstResponseByte: func() { ttfb = time.Since(reqStart) },
		}

		req, err := http.NewRequestWithContext(
			httptrace.WithClientTrace(ctx, trace), method, rawURL, nil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set Range header
		hasRange := false
		if rangeStart != 0 || rangeEnd != 0 {
			hasRange = true
			if rangeStart < 0 {
				// Suffix range: last N bytes
				req.Header.Set("Range", fmt.Sprintf("bytes=%d", rangeStart))
			} else if rangeEnd > 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
			} else {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
			}
		}

		reqStart = time.Now()
		resp, err := client.Do(req)
		totalDur := time.Since(reqStart)

		if err != nil {
			return map[string]interface{}{
				"error":          err.Error(),
				"dns_ms":         dnsDur.Milliseconds(),
				"tcp_connect_ms": connDur.Milliseconds(),
				"tls_ms":         tlsDur.Milliseconds(),
				"total_ms":       totalDur.Milliseconds(),
			}, nil
		}
		defer resp.Body.Close()

		// Consume limited body to get accurate total time
		bodyBytes, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))

		// Collect response headers
		headers := map[string]string{}
		for _, key := range []string{
			"Content-Type", "Content-Length", "Content-Range",
			"Accept-Ranges", "ETag", "Last-Modified",
			"X-Served-By", "Server",
		} {
			if v := resp.Header.Get(key); v != "" {
				headers[key] = v
			}
		}

		// TLS info
		tlsInfo := map[string]interface{}{}
		if resp.TLS != nil {
			tlsInfo["version"] = tlsVersionName(resp.TLS.Version)
			tlsInfo["cipher"] = tls.CipherSuiteName(resp.TLS.CipherSuite)
			tlsInfo["server_name"] = resp.TLS.ServerName
			tlsInfo["negotiated_protocol"] = resp.TLS.NegotiatedProtocol
		}

		result := map[string]interface{}{
			"status":         resp.StatusCode,
			"status_text":    resp.Status,
			"proto":          resp.Proto,
			"headers":        headers,
			"body_bytes":     bodyBytes,
			"has_range":      hasRange,
			"dns_ms":         dnsDur.Milliseconds(),
			"tcp_connect_ms": connDur.Milliseconds(),
			"tls_ms":         tlsDur.Milliseconds(),
			"ttfb_ms":        ttfb.Milliseconds(),
			"total_ms":       totalDur.Milliseconds(),
		}

		if len(tlsInfo) > 0 {
			result["tls"] = tlsInfo
		}

		// Add diagnostic hints
		if resp.StatusCode == 200 && hasRange {
			result["hint"] = "Server returned 200 instead of 206 — Range requests may not be supported for this path"
		}
		if resp.Header.Get("Accept-Ranges") == "" && method == "HEAD" {
			result["hint"] = "No Accept-Ranges header — server may not advertise Range support"
		}

		return result, nil
	})
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (0x%04x)", v)
	}
}
