package main

import (
	"testing"
)

func TestEnsureObjectstoreArgsStrictNumeric(t *testing.T) {
	args, err := buildObjectstoreArgs("/tmp/contract.json", "example.com", 30, 1000, true)
	if err != nil {
		t.Fatalf("buildObjectstoreArgs returned error: %v", err)
	}
	fields := args.GetFields()
	if fields["strict_contract"].GetBoolValue() != true {
		t.Fatalf("strict_contract expected true")
	}
	if fields["retry"].GetNumberValue() != 30 {
		t.Fatalf("retry should be numeric 30, got %v", fields["retry"].GetNumberValue())
	}
	if fields["retry_delay_ms"].GetNumberValue() != 1000 {
		t.Fatalf("retry_delay_ms should be numeric 1000, got %v", fields["retry_delay_ms"].GetNumberValue())
	}
	if fields["domain"].GetStringValue() != "example.com" {
		t.Fatalf("domain mismatch")
	}
}
