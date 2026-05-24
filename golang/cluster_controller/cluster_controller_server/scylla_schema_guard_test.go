package main

import "testing"

func TestCriticalScyllaKeyspacesIncludesRepository(t *testing.T) {
	found := false
	for _, ks := range criticalScyllaKeyspaces {
		if ks == "repository" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("criticalScyllaKeyspaces must include repository for RF policy enforcement")
	}
}

