package main

import "testing"

func TestHasNonEmptyAuthToken(t *testing.T) {
	if hasNonEmptyAuthToken("auth_token: abc123\n") != true {
		t.Fatal("expected true for non-empty token")
	}
	if hasNonEmptyAuthToken("auth_token: \n") != false {
		t.Fatal("expected false for empty token")
	}
	if hasNonEmptyAuthToken("# comment\nauth_token: \"tok\"\n") != true {
		t.Fatal("expected true for quoted token")
	}
}

func TestUpsertAuthToken(t *testing.T) {
	const token = "newtoken"

	got := upsertAuthToken("https: :10001\n", token)
	if !hasNonEmptyAuthToken(got) {
		t.Fatalf("upsert should add auth_token, got:\n%s", got)
	}

	got = upsertAuthToken("auth_token: old\nhttps: :10001\n", token)
	if got == "" || got[:len("auth_token: newtoken")] != "auth_token: newtoken" {
		t.Fatalf("upsert should replace existing token, got:\n%s", got)
	}
}
