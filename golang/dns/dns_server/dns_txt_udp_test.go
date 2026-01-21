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
