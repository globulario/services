package rules

import (
	"errors"
	"testing"
)

func TestMapCheckErr(t *testing.T) {
	if got := mapCheckErr(nil); got != "" {
		t.Fatalf("mapCheckErr(nil) = %q, want empty", got)
	}
	if got := mapCheckErr(errors.New("boom")); got != InvariantStateCheckError {
		t.Fatalf("mapCheckErr(err) = %q, want %q", got, InvariantStateCheckError)
	}
}
