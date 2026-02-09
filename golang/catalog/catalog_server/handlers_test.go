package main

import "testing"

func TestGetOptionsStringAddsIdProjection(t *testing.T) {
    srv := initializeServerDefaults()

    got, err := srv.getOptionsString(``)
    if err != nil {
        t.Fatalf("getOptionsString error = %v", err)
    }
    if got == "" {
        t.Fatal("getOptionsString returned empty string")
    }
    if got == "[]" {
        t.Fatal("expected projection to be injected")
    }
}
