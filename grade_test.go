package main

import (
	"strings"
	"testing"
)

// The real report that prompted auto detection of routed / VPN links:
// two wired gigabit PCs on different subnets joined by a site-to-site VPN.
func siteToSiteVPN() *Result {
	return &Result{
		LocalHost: "Boathouse2", LocalIP: "192.168.110.10", LocalIface: "Ethernet 5", LocalKind: "wired", LocalLinkMbps: 1000,
		RemoteHost: "Nipissing", RemoteIP: "192.168.113.99", RemoteIface: "Ethernet", RemoteKind: "wired", RemoteLinkMbps: 1000,
		Route:    "routed",
		DownMbps: 25.2, UpMbps: 31.0, IdleRTT: 27.4, IdleRTTMin: 19.2, IdleRTTMax: 38.9, JitterMs: 3.76,
		Sent: 300, LatencyMethod: "UDP", DownLoadedRTT: 31.9, UpLoadedRTT: 36.6,
	}
}

func TestAutoDetectsSiteToSiteVPN(t *testing.T) {
	r := siteToSiteVPN()
	r.Grade("auto")
	if r.ProfileKey != "vpn" {
		t.Fatalf("auto picked %q, want vpn (reason %q)", r.ProfileKey, r.AutoReason)
	}
	if !strings.Contains(r.AutoReason, "different network") {
		t.Errorf("reason should explain the different network: %q", r.AutoReason)
	}
	if r.Verdict == "poor" {
		t.Errorf("a 27 ms VPN with no loss should not be graded poor:\n%s", r.Text())
	}
	t.Log("\n" + r.Text())
}

func TestUserChoiceOverridesAuto(t *testing.T) {
	r := siteToSiteVPN()
	r.Grade("1g")
	if r.ProfileKey != "1g" || r.Verdict != "poor" {
		t.Fatalf("forcing 1g should grade poor, got %s/%s", r.ProfileKey, r.Verdict)
	}
	found := false
	for _, f := range r.Findings {
		if strings.Contains(f, "different network") {
			found = true
		}
	}
	if !found {
		t.Errorf("should hint that the link looks like a VPN")
	}
}

func TestSameSubnetGigabitStaysLAN(t *testing.T) {
	r := &Result{LocalKind: "wired", RemoteKind: "wired", LocalLinkMbps: 1000, RemoteLinkMbps: 1000,
		RemoteIP: "192.168.1.20", Route: "local", IdleRTT: 0.3, DownMbps: 300, UpMbps: 320, LatencyMethod: "UDP", Sent: 300}
	r.Grade("auto")
	if r.ProfileKey != "1g" || r.Verdict != "poor" {
		t.Fatalf("slow same-subnet gigabit must be flagged against 1g, got %s/%s", r.ProfileKey, r.Verdict)
	}
}

func TestHighDelayOnSameSubnetMeansNotLocal(t *testing.T) {
	// Layer 2 VPN / bridged tunnel: same subnet but 20 ms away.
	r := &Result{LocalKind: "wired", RemoteKind: "wired", RemoteIP: "10.0.0.5", Route: "local", IdleRTT: 20, LatencyMethod: "UDP", Sent: 300}
	r.Grade("auto")
	if r.ProfileKey != "vpn" {
		t.Fatalf("got %s", r.ProfileKey)
	}
}
