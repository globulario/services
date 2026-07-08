package main

import "testing"

func TestSplitDesiredServiceIdentity(t *testing.T) {
	tests := []struct {
		raw           string
		wantPublisher string
		wantName      string
	}{
		{raw: "event", wantPublisher: "", wantName: "event"},
		{raw: "local@globule-ryzen/event", wantPublisher: "local@globule-ryzen", wantName: "event"},
		{raw: "core@globular.io/cluster-controller", wantPublisher: "core@globular.io", wantName: "cluster-controller"},
	}

	for _, tt := range tests {
		gotPublisher, gotName := splitDesiredServiceIdentity(tt.raw)
		if gotPublisher != tt.wantPublisher || gotName != tt.wantName {
			t.Fatalf("splitDesiredServiceIdentity(%q) = (%q, %q), want (%q, %q)", tt.raw, gotPublisher, gotName, tt.wantPublisher, tt.wantName)
		}
	}
}
