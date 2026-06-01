package actions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

type httpProbeAction struct{}

func (httpProbeAction) Name() string { return "probe.http" }

func (httpProbeAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["url"].GetStringValue() == "" {
		return errors.New("url is required")
	}
	return nil
}

func (httpProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	url := strings.TrimSpace(args.GetFields()["url"].GetStringValue())
	method := strings.ToUpper(strings.TrimSpace(args.GetFields()["method"].GetStringValue()))
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", err
	}
	timeout := 5 * time.Second
	if to := args.GetFields()["timeout_ms"].GetNumberValue(); to > 0 {
		timeout = time.Duration(int64(to)) * time.Millisecond
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return fmt.Sprintf("http %d", resp.StatusCode), nil
}

type tcpProbeAction struct{}

func (tcpProbeAction) Name() string { return "probe.tcp" }

func (tcpProbeAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["address"].GetStringValue() == "" {
		return errors.New("address is required")
	}
	return nil
}

func (tcpProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	address := strings.TrimSpace(args.GetFields()["address"].GetStringValue())
	timeout := 5 * time.Second
	if to := args.GetFields()["timeout_ms"].GetNumberValue(); to > 0 {
		timeout = time.Duration(int64(to)) * time.Millisecond
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	conn.Close()
	return fmt.Sprintf("tcp %s ok", address), nil
}

func init() {
	Register(httpProbeAction{})
	Register(tcpProbeAction{})
}
