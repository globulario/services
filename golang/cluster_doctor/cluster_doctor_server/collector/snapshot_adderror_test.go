package collector

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

// TestAddError_RecordsAndLogs locks in the fix for the silent-harvest bug:
// per-node collector fetch failures (GetInventory, GetInfraProbe, …) funnel
// through addError, which previously recorded the error to DataErrors but never
// logged it — so a reduced-harvest sweep had no recorded cause. addError must
// now (a) record the error and mark the snapshot incomplete, and (b) log the
// underlying error so timeout vs auth vs unimplemented is distinguishable.
// (meta.connection_errors_must_not_be_absorbed)
func TestAddError_RecordsAndLogs(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(prevFlags) })

	s := &Snapshot{}
	s.addError("node_agent@n1", "GetInventory", errors.New("context deadline exceeded"))

	if !s.DataIncomplete {
		t.Error("addError must set DataIncomplete")
	}
	if !s.HadError("node_agent@n1", "GetInventory") {
		t.Error("addError must record the error so HadError(service, rpc) reports it")
	}
	out := buf.String()
	for _, want := range []string{"reduced harvest", "GetInventory", "context deadline exceeded"} {
		if !strings.Contains(out, want) {
			t.Errorf("addError must log %q; got %q", want, out)
		}
	}
}
