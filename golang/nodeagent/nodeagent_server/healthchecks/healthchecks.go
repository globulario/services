package healthchecks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Check struct {
	Name           string
	URL            string
	InsecureTLS    bool
	ExpectedStatus []int
	Timeout        time.Duration
	HostHeader     string
}

func RunChecks(ctx context.Context, checks []Check) error {
	var errs []string
	client := &http.Client{}
	for _, c := range checks {
		statusOK := make(map[int]struct{}, len(c.ExpectedStatus))
		for _, s := range c.ExpectedStatus {
			statusOK[s] = struct{}{}
		}
		attempts := 30
		var lastErr error
		for i := 0; i < attempts; i++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
			if err != nil {
				lastErr = err
				break
			}
			if c.HostHeader != "" {
				req.Host = c.HostHeader
				req.Header.Set("Host", c.HostHeader)
			}
			t := &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: time.Second * 5,
				}).DialContext,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureTLS}, //nolint:gosec
			}
			client.Timeout = time.Duration(c.Timeout)
			client.Transport = t
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
			} else {
				resp.Body.Close()
				if _, ok := statusOK[resp.StatusCode]; ok {
					lastErr = nil
					break
				}
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
			}
			time.Sleep(time.Second)
		}
		if lastErr != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name, lastErr))
		}
	}
	if len(errs) > 0 {
		return errors.New("checks failed: " + fmt.Sprint(errs))
	}
	return nil
}
