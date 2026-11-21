package main

import (
	"strings"
	"testing"
)

// The case from a real test: a wired Windows PC and a Linux laptop on Wi-Fi,
// same subnet, lag when busy and some jitter, no loss.
func wifiInHouse() *Result {
	return &Result{
		LocalHost: "Nipissing", LocalOS: "windows", LocalIface: "Ethernet", LocalKind: "wired", LocalLinkMbps: 1000,
		RemoteHost: "mike-mint", RemoteOS: "linux", RemoteIP: "192.168.113.129", RemoteIface: "wlp1s0", RemoteKind: "wifi",
		Route: "local", DownMbps: 41, UpMbps: 49, IdleRTT: 7.6, IdleRTTMin: 2, IdleRTTMax: 40, JitterMs: 9,
		Sent: 300, LatencyMethod: "UDP", DownLoadedRTT: 120.8, UpLoadedRTT: 97,
		UDPRateMbps: 20,
		UDPDown:     &UDPStats{Sent: 16666, Received: 16666, JitterMs: 6, Mbps: 20},
		UDPUp:       &UDPStats{Sent: 16666, Received: 16666, JitterMs: 5, Mbps: 20},
	}
}

func fixText(r *Result) string {
	var b strings.Builder
	for _, f := range r.Fixes {
		b.WriteString(f.Title + "\n  " + strings.Join(f.Steps, "\n  ") + "\n")
	}
	return b.String()
}

func TestWifiAdviceFitsTheComputers(t *testing.T) {
	r := wifiInHouse()
	r.Grade("auto")
	if r.ProfileKey != "wifi" {
		t.Fatalf("profile %s", r.ProfileKey)
	}
	all := fixText(r)
	t.Log("\n" + all)

	// Same subnet: the internet line was never tested, so no router SQM.
	if strings.Contains(all, "SQM / Smart Queue") || strings.Contains(all, "90% of your real line speed") {
		t.Error("internet SQM advice for a test that stayed in the house")
	}
	if !strings.Contains(all, "Stop the Wi-Fi getting laggy") {
		t.Error("missing the Wi-Fi bufferbloat fix")
	}
	// Only mike-mint is on Wi-Fi, and it runs Linux: no Windows steps, and
	// its real adapter name in the command.
	for _, w := range []string{"Power Options", "Roaming Aggressiveness", "Device Manager"} {
		if strings.Contains(all, w) {
			t.Errorf("Windows step %q for a Linux Wi-Fi computer", w)
		}
	}
	if !strings.Contains(all, "sudo iw dev wlp1s0 set power_save off") || !strings.Contains(all, "On mike-mint (Linux)") {
		t.Error("missing the Linux steps for mike-mint")
	}
	// No loss: jitter advice must not say to fix loss first.
	if strings.Contains(all, "packet loss") {
		t.Error("talks about packet loss when there was none")
	}
}

// Over the internet, bufferbloat advice is still about the router's SQM.
func TestBloatOverVPNStillSuggestsSQM(t *testing.T) {
	r := wifiInHouse()
	r.Route, r.RemoteIP, r.RemoteKind, r.IdleRTT = "routed", "100.92.218.56", "wired", 30
	r.Grade("auto")
	if all := fixText(r); !strings.Contains(all, "SQM / Smart Queue") {
		t.Errorf("no SQM advice over the internet:\n%s", all)
	}
}
