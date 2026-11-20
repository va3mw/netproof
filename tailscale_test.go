//go:build !windows

package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTailscale puts a "tailscale" command on PATH that prints a status.
func fakeTailscale(t *testing.T) {
	dir := t.TempDir()
	js := `{"Peer":{"k1":{"HostName":"Boathouse2","DNSName":"boathouse2.tail1234.ts.net.","TailscaleIPs":["100.92.218.56","fd7a:115c:a1e0::1"],"Online":true},
	"k2":{"HostName":"radio-pc","DNSName":"radio-pc.tail1234.ts.net.","TailscaleIPs":["127.0.0.2"],"Online":true}}}`
	if err := os.WriteFile(filepath.Join(dir, "tailscale"), []byte("#!/bin/sh\ncat <<'X'\n"+js+"\nX\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestTailscaleNames(t *testing.T) {
	fakeTailscale(t)
	ctx := context.Background()
	for _, name := range []string{"boathouse2", "BOATHOUSE2", "boathouse2.tail1234.ts.net", "boathouse2.tail1234.ts.net."} {
		ip, fqdn, ok := tailscaleLookup(ctx, name)
		if !ok || ip != "100.92.218.56" || fqdn != "boathouse2.tail1234.ts.net" {
			t.Errorf("%s -> %s %s %v", name, ip, fqdn, ok)
		}
	}
	if _, _, ok := tailscaleLookup(ctx, "nosuchbox"); ok {
		t.Error("unknown name matched")
	}
	// Not in DNS, found through Tailscale.
	ip, name, err := resolveTarget(ctx, "boathouse2")
	if err != nil || ip != "100.92.218.56" || name != "boathouse2.tail1234.ts.net" {
		t.Errorf("resolveTarget: %s %s %v", ip, name, err)
	}
	if _, _, err := resolveTarget(ctx, "nosuchbox"); err == nil || !strings.Contains(err.Error(), "Tailscale") {
		t.Errorf("want helpful error, got %v", err)
	}
}

func TestTailscaleIPv6Recognised(t *testing.T) {
	if !isTailscaleIP(net.ParseIP("fd7a:115c:a1e0::1")) || !isTailscaleIP(net.ParseIP("100.92.218.56")) || isTailscaleIP(net.ParseIP("192.168.1.1")) {
		t.Fatal("tailscale range detection wrong")
	}
	r := &Result{RemoteIP: "fd7a:115c:a1e0::1", LocalKind: "wired", RemoteKind: "wired", LatencyMethod: "UDP", Sent: 1}
	r.Grade("auto")
	if r.ProfileKey != "vpn" {
		t.Fatalf("got %s", r.ProfileKey)
	}
}
