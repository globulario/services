package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// config.GetEtcdClient() returns the process-wide SHARED singleton.
// config.NewEtcdClient() is the caller-owned variant that must be closed.
//
// Closing the shared client tears down etcd access for every other caller in
// the controller. Observed 2026-08-11 on 1.2.310: the invariant lane called
// clearQuorumLossAlert every 60s on a healthy cluster, each call did
// `defer cli.Close()` on the shared client, and the controller then spun
// continuously on
//
//	{"logger":"etcd-client","msg":"retrying of unary invoker failed",
//	 "error":"rpc error: code = Canceled desc = grpc: the client connection is closing"}
//
// with every cluster RPC failing DeadlineExceeded and the cluster never
// reaching ready. The same shape was already latent in writeQuorumLossAlert;
// it only became reachable once the enforcement workflow started running.
//
// A short-lived CLI closing it at exit is harmless; a long-running service
// doing so is not. This file guards the controller.

func TestControllerNeverClosesSharedEtcdClient(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	getShared := regexp.MustCompile(`GetEtcdClient\s*\(\s*\)`)
	closeCall := regexp.MustCompile(`defer\s+\w+\.Close\s*\(\s*\)`)

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		text := string(src)
		for _, loc := range getShared.FindAllStringIndex(text, -1) {
			// Look ahead a short window for a deferred Close on the result.
			end := loc[1] + 400
			if end > len(text) {
				end = len(text)
			}
			window := text[loc[1]:end]
			if closeCall.MatchString(window) {
				t.Errorf("%s closes the SHARED etcd client near offset %d — this "+
					"tears down etcd for the whole controller. Use "+
					"config.NewEtcdClient() if you need an owned client, or do "+
					"not close it.", name, loc[0])
			}
		}
	}
}
