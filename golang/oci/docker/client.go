package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DefaultSocket = "unix:///var/run/docker.sock"

type Client struct {
	httpClient *http.Client
	baseURL    string
	endpoint   string

	mu         sync.Mutex
	apiVersion string
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("docker API returned HTTP %d: %s", e.StatusCode, e.Message)
}

func IsNotFound(err error) bool {
	he, ok := err.(*HTTPError)
	return ok && he.StatusCode == http.StatusNotFound
}

func NewClient(endpoint string) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = DefaultSocket
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	baseURL := "http://docker"
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		socket := filepath.Clean(strings.TrimPrefix(endpoint, "unix://"))
		if !filepath.IsAbs(socket) {
			return nil, fmt.Errorf("docker socket path must be absolute: %q", socket)
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}
	case strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://"):
		baseURL = strings.TrimRight(endpoint, "/")
	default:
		return nil, fmt.Errorf("unsupported Docker endpoint %q; use unix://, http://, or https://", endpoint)
	}
	return &Client{
		httpClient: &http.Client{Transport: transport},
		baseURL:    baseURL,
		endpoint:   endpoint,
	}, nil
}

func (c *Client) Endpoint() string { return c.endpoint }

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.request(ctx, http.MethodGet, "/_ping", nil, nil, false)
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if strings.TrimSpace(string(b)) != "OK" {
		return fmt.Errorf("unexpected Docker ping response %q", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) APIVersion(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.apiVersion != "" {
		return c.apiVersion, nil
	}
	resp, err := c.request(ctx, http.MethodGet, "/version", nil, nil, false)
	if err != nil {
		return "", err
	}
	defer closeQuietly(resp.Body)
	var v struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", fmt.Errorf("decode docker version: %w", err)
	}
	if strings.TrimSpace(v.APIVersion) == "" {
		return "", fmt.Errorf("docker version response omitted ApiVersion")
	}
	c.apiVersion = strings.TrimPrefix(strings.TrimSpace(v.APIVersion), "v")
	return c.apiVersion, nil
}

func (c *Client) Do(ctx context.Context, method, path string, body any, headers map[string]string) (*http.Response, error) {
	return c.request(ctx, method, path, body, headers, true)
}

func (c *Client) request(ctx context.Context, method, path string, body any, headers map[string]string, versioned bool) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode docker request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	if versioned {
		version, err := c.APIVersion(ctx)
		if err != nil {
			return nil, err
		}
		path = "/v" + version + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker API %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer closeQuietly(resp.Body)
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	message := strings.TrimSpace(string(b))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &payload) == nil && payload.Message != "" {
		message = payload.Message
	}
	return nil, &HTTPError{StatusCode: resp.StatusCode, Message: message}
}

func closeQuietly(closer io.Closer) {
	_ = closer.Close()
}

func queryPath(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
