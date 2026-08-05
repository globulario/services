package oci

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func WaitProbe(ctx context.Context, probe Probe) error {
	if strings.TrimSpace(probe.Type) == "" || strings.EqualFold(probe.Type, "none") {
		return nil
	}
	initial := durationMillis(probe.InitialDelayMillis, 0)
	if initial > 0 {
		t := time.NewTimer(initial)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	interval := durationMillis(probe.IntervalMillis, time.Second)
	threshold := probe.FailureThreshold
	if threshold <= 0 {
		threshold = 30
	}
	var last error
	for i := 0; i < threshold; i++ {
		if err := CheckProbe(ctx, probe); err == nil {
			return nil
		} else {
			last = err
		}
		if i+1 == threshold {
			break
		}
		t := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return fmt.Errorf("probe failed after %d attempts: %w", threshold, last)
}

func CheckProbe(ctx context.Context, probe Probe) error {
	typ := strings.ToLower(defaultString(probe.Type, "none"))
	if typ == "none" {
		return nil
	}
	address := strings.TrimSpace(probe.Address)
	timeout := durationMillis(probe.TimeoutMillis, 2*time.Second)
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint := net.JoinHostPort(address, fmt.Sprintf("%d", probe.Port))
	if typ == "tcp" {
		d := net.Dialer{}
		conn, err := d.DialContext(probeCtx, "tcp", endpoint)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	if typ != "http" {
		return fmt.Errorf("unsupported probe type %q", typ)
	}
	scheme := defaultString(probe.Scheme, "http")
	path := defaultString(probe.Path, "/")
	tr := &http.Transport{
		DialContext:     (&net.Dialer{Timeout: timeout}).DialContext,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, scheme+"://"+endpoint+path, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	min := probe.ExpectedStatusMin
	max := probe.ExpectedStatusMax
	if min == 0 {
		min = 200
	}
	if max == 0 {
		max = 399
	}
	if resp.StatusCode < min || resp.StatusCode > max {
		return fmt.Errorf("unexpected HTTP status %d (wanted %d-%d)", resp.StatusCode, min, max)
	}
	return nil
}

func durationMillis(v int64, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return time.Duration(v) * time.Millisecond
}
