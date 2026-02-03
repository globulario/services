package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/globulario/services/golang/dns/dnspb"
)

func TestTXTServedOverUDP(t *testing.T) {
	tmpDir := t.TempDir()
	s := &server{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Root:   tmpDir,
	}
	srv = s
	if err := s.openConnection(); err != nil {
		t.Fatalf("open connection: %v", err)
	}

	if _, err := s.SetDomains(context.Background(), &dnspb.SetDomainsRequest{Domains: []string{"globular.io"}}); err != nil {
		t.Fatalf("SetDomains: %v", err)
	}
	if _, err := s.SetText(context.Background(), &dnspb.SetTextRequest{Id: "_acme-challenge.globular.io", Values: []string{"token-value"}, Ttl: 120}); err != nil {
		t.Fatalf("SetText: %v", err)
	}

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("udp listen not permitted: %v", err)
	}
	defer pc.Close()

	dnsSrv := &dns.Server{PacketConn: pc, Handler: &handler{}}
	go dnsSrv.ActivateAndServe()
	defer dnsSrv.Shutdown()

	time.Sleep(50 * time.Millisecond)

	client := dns.Client{}
	msg := new(dns.Msg)
	msg.SetQuestion("_acme-challenge.globular.io.", dns.TypeTXT)

	res, _, err := client.Exchange(msg, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("dns exchange: %v", err)
	}
	found := false
	for _, ans := range res.Answer {
		if txt, ok := ans.(*dns.TXT); ok {
			for _, v := range txt.Txt {
				if v == "token-value" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected TXT answer token-value, got %#v", res.Answer)
	}
}

func TestTXTManagedDomainEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	s := &server{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Root:   tmpDir,
	}
	srv = s
	if err := s.openConnection(); err != nil {
		t.Fatalf("open connection: %v", err)
	}

	// Set managed domain
	if _, err := s.SetDomains(context.Background(), &dnspb.SetDomainsRequest{Domains: []string{"globular.io"}}); err != nil {
		t.Fatalf("SetDomains: %v", err)
	}

	// Test SetTXT for managed domain - should succeed
	if _, err := s.SetTXT(context.Background(), &dnspb.SetTXTRequest{Domain: "_acme-challenge.globular.io", Txt: "test-token", Ttl: 300}); err != nil {
		t.Fatalf("SetTXT for managed domain: %v", err)
	}

	// Test SetTXT for unmanaged domain - should fail
	if _, err := s.SetTXT(context.Background(), &dnspb.SetTXTRequest{Domain: "_acme-challenge.unmanaged.com", Txt: "test-token", Ttl: 300}); err == nil {
		t.Fatalf("SetTXT for unmanaged domain should fail")
	}

	// Test GetTXT
	resp, err := s.GetTXT(context.Background(), &dnspb.GetTXTRequest{Domain: "_acme-challenge.globular.io"})
	if err != nil {
		t.Fatalf("GetTXT: %v", err)
	}
	if len(resp.Txt) != 1 || resp.Txt[0] != "test-token" {
		t.Fatalf("expected [test-token], got %v", resp.Txt)
	}

	// Test RemoveTXT with specific value
	if _, err := s.SetTXT(context.Background(), &dnspb.SetTXTRequest{Domain: "_acme-challenge.globular.io", Txt: "test-token-2", Ttl: 300}); err != nil {
		t.Fatalf("SetTXT second value: %v", err)
	}

	resp, _ = s.GetTXT(context.Background(), &dnspb.GetTXTRequest{Domain: "_acme-challenge.globular.io"})
	if len(resp.Txt) != 2 {
		t.Fatalf("expected 2 TXT values, got %d", len(resp.Txt))
	}

	// Remove specific value
	if _, err := s.RemoveTXT(context.Background(), &dnspb.RemoveTXTRequest{Domain: "_acme-challenge.globular.io", Txt: "test-token"}); err != nil {
		t.Fatalf("RemoveTXT: %v", err)
	}

	resp, _ = s.GetTXT(context.Background(), &dnspb.GetTXTRequest{Domain: "_acme-challenge.globular.io"})
	if len(resp.Txt) != 1 || resp.Txt[0] != "test-token-2" {
		t.Fatalf("expected [test-token-2], got %v", resp.Txt)
	}

	// Remove all TXT values
	if _, err := s.RemoveTXT(context.Background(), &dnspb.RemoveTXTRequest{Domain: "_acme-challenge.globular.io", Txt: ""}); err != nil {
		t.Fatalf("RemoveTXT all: %v", err)
	}

	resp, _ = s.GetTXT(context.Background(), &dnspb.GetTXTRequest{Domain: "_acme-challenge.globular.io"})
	if len(resp.Txt) != 0 {
		t.Fatalf("expected empty TXT values, got %v", resp.Txt)
	}
}

func TestTXTNewMethodServedOverUDP(t *testing.T) {
	tmpDir := t.TempDir()
	s := &server{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Root:   tmpDir,
	}
	srv = s
	if err := s.openConnection(); err != nil {
		t.Fatalf("open connection: %v", err)
	}

	if _, err := s.SetDomains(context.Background(), &dnspb.SetDomainsRequest{Domains: []string{"globular.io"}}); err != nil {
		t.Fatalf("SetDomains: %v", err)
	}

	// Use new SetTXT method
	if _, err := s.SetTXT(context.Background(), &dnspb.SetTXTRequest{Domain: "_acme-challenge.globular.io", Txt: "new-token-value", Ttl: 120}); err != nil {
		t.Fatalf("SetTXT: %v", err)
	}

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("udp listen not permitted: %v", err)
	}
	defer pc.Close()

	dnsSrv := &dns.Server{PacketConn: pc, Handler: &handler{}}
	go dnsSrv.ActivateAndServe()
	defer dnsSrv.Shutdown()

	time.Sleep(50 * time.Millisecond)

	client := dns.Client{}
	msg := new(dns.Msg)
	msg.SetQuestion("_acme-challenge.globular.io.", dns.TypeTXT)

	res, _, err := client.Exchange(msg, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("dns exchange: %v", err)
	}
	found := false
	for _, ans := range res.Answer {
		if txt, ok := ans.(*dns.TXT); ok {
			for _, v := range txt.Txt {
				if v == "new-token-value" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected TXT answer new-token-value, got %#v", res.Answer)
	}
}
