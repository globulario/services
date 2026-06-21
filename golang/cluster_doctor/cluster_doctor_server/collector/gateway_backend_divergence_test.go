package collector

import (
	"errors"
	"testing"
)

// classifyProbeErr must recognise the distinctive misrouting signal: a gRPC
// client that got an HTML response instead of a gRPC one.
func TestClassifyProbeErr_DetectsHTMLContentType(t *testing.T) {
	err := errors.New(`rpc error: code = Unknown desc = unexpected HTTP status code received from server: 200 (OK); transport: received unexpected content-type "text/html"`)
	r := classifyProbeErr(err)
	if !r.html {
		t.Fatalf("expected html=true for a text/html transport error")
	}
	if r.contentType != "text/html" {
		t.Errorf("contentType=%q, want text/html", r.contentType)
	}
	if r.reachable {
		t.Errorf("an html misroute is not reachable gRPC")
	}
}

// A plain unavailable must NOT be classified as an HTML misroute.
func TestClassifyProbeErr_PlainUnavailableIsNotHTML(t *testing.T) {
	r := classifyProbeErr(errors.New("rpc error: code = Unavailable desc = connection refused"))
	if r.html {
		t.Errorf("connection refused must not be classified as html")
	}
	if r.reachable {
		t.Errorf("an errored probe is not reachable")
	}
}

// classifyGatewayBackend folds two probe results into a snapshot record faithfully.
func TestClassifyGatewayBackend_FoldsResults(t *testing.T) {
	gw := probeResult{html: true, contentType: "text/html", err: errors.New(`content-type "text/html"`)}
	be := probeResult{reachable: true}
	p := classifyGatewayBackend("ai_memory.AiMemoryService", "gw:443", "be:10009", gw, be, 1700000000)

	if p.Service != "ai_memory.AiMemoryService" || p.GatewayEndpoint != "gw:443" || p.BackendEndpoint != "be:10009" {
		t.Errorf("identity fields not preserved: %+v", p)
	}
	if !p.GatewayHTML || p.GatewayContentType != "text/html" || p.GatewayReachable {
		t.Errorf("gateway fields wrong: %+v", p)
	}
	if !p.BackendChecked || !p.BackendReachable {
		t.Errorf("backend fields wrong: %+v", p)
	}
	if p.GatewayErr == "" {
		t.Errorf("gateway error should be recorded")
	}
	if p.ObservedAtUnix != 1700000000 {
		t.Errorf("timestamp not preserved")
	}
}

// No backend endpoint => BackendChecked is false (drives the indeterminate rule path).
func TestClassifyGatewayBackend_NoBackendEndpointIsUnchecked(t *testing.T) {
	p := classifyGatewayBackend("svc", "gw:443", "", probeResult{html: true, contentType: "text/html"}, probeResult{err: errors.New("backend endpoint unresolved")}, 1)
	if p.BackendChecked {
		t.Errorf("empty backend endpoint must mark BackendChecked=false")
	}
}
