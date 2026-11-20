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
		RemoteIP: "192.168.1.20", Route: "local", IdleRTT: 0.3, DownMbps: 150, UpMbps: 160, LatencyMethod: "UDP", Sent: 300}
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

// A sick gigabit LAN: one PC stuck at 100 Mb/s, UDP stream loss, bufferbloat,
// a small MTU and fragments dropped. Every problem must get a fix.
func sickLAN() *Result {
	return &Result{
		LocalHost: "Shack", LocalIface: "Ethernet", LocalKind: "wired", LocalLinkMbps: 1000,
		RemoteHost: "Radio-PC", RemoteIP: "192.168.1.20", RemoteIface: "eth0", RemoteKind: "wired", RemoteLinkMbps: 100,
		Route: "local", DownMbps: 92, UpMbps: 40, IdleRTT: 0.4, IdleRTTMin: 0.2, IdleRTTMax: 3, JitterMs: 0.3,
		Sent: 300, LatencyMethod: "UDP", DownLoadedRTT: 45, UpLoadedRTT: 20,
		UDPRateMbps: 28,
		UDPDown:     &UDPStats{Sent: 23333, Received: 23100, LossPct: 1.0, JitterMs: 0.4, Mbps: 27.7},
		UDPUp:       &UDPStats{Sent: 23333, Received: 23333, JitterMs: 0.2, Mbps: 28},
		MTU:         &MTUResult{PathMTU: 1400, DFSupported: true, FragTested: true, FragmentsOK: false},
	}
}

func TestSickLANGetsFixes(t *testing.T) {
	r := sickLAN()
	r.Grade("auto") // one end gigabit: must be judged as a gigabit LAN
	if r.ProfileKey != "1g" {
		t.Fatalf("auto picked %s", r.ProfileKey)
	}
	if r.Verdict != "poor" {
		t.Fatalf("verdict %s", r.Verdict)
	}
	want := []string{"connected at 1 Gb/s", "packets are being lost", "fragmented"}
	var titles []string
	for _, f := range r.Fixes {
		titles = append(titles, f.Title)
	}
	all := strings.Join(titles, " | ")
	for _, w := range want {
		if !strings.Contains(all, w) {
			t.Errorf("missing fix %q in: %s", w, all)
		}
	}
	// 44.6 ms busy delay on a LAN and MTU 1400 are not things you notice.
	if gradeOf(r.Metrics, "udploss") != "poor" || gradeOf(r.Metrics, "mtu") != "fair" || gradeOf(r.Metrics, "bloat") != "good" {
		t.Errorf("udploss=%s mtu=%s bloat=%s", gradeOf(r.Metrics, "udploss"), gradeOf(r.Metrics, "mtu"), gradeOf(r.Metrics, "bloat"))
	}
	t.Log("\n" + r.Text())
}

func TestHealthyLANHasNoFixes(t *testing.T) {
	r := &Result{LocalKind: "wired", RemoteKind: "wired", LocalLinkMbps: 1000, RemoteLinkMbps: 1000, Route: "local",
		DownMbps: 941, UpMbps: 938, IdleRTT: 0.3, JitterMs: 0.05, Sent: 300, LatencyMethod: "UDP",
		DownLoadedRTT: 1.2, UpLoadedRTT: 1.1, UDPRateMbps: 100,
		UDPDown: &UDPStats{Sent: 83333, Received: 83333, JitterMs: 0.02},
		UDPUp:   &UDPStats{Sent: 83333, Received: 83333, JitterMs: 0.02},
		MTU:     &MTUResult{PathMTU: 1500, DFSupported: true, FragTested: true, FragmentsOK: true}}
	r.Grade("auto")
	if r.Verdict != "good" || len(r.Fixes) != 0 {
		t.Fatalf("verdict %s, fixes %+v", r.Verdict, r.Fixes)
	}
}

func TestStreamRxCounts(t *testing.T) {
	rx := newStreamRx()
	now := rx.ref
	for _, s := range []uint32{0, 1, 2, 4, 3, 3, 6, 7, 8, 9} { // seq 5 lost, one duplicate 3, one out of order 3
		now = now.Add(1000000)
		rx.add(s, int64(s)*1000000, 1200, now)
	}
	st := rx.stats(10)
	if st.Received != 9 || st.OutOfOrder != 1 || st.LossPct != 10 {
		t.Fatalf("%+v", st)
	}
}

func TestUDPRateNeverOverrunsLink(t *testing.T) {
	if r := udpRateFor("vpn", 25, 31); r != 10 {
		t.Errorf("vpn 25/31 -> %v", r)
	}
	if r := udpRateFor("vpn", 8, 31); r != 5.6 {
		t.Errorf("vpn 8/31 -> %v", r)
	}
	if r := udpRateFor("1g", 940, 935); r != 100 {
		t.Errorf("1g -> %v", r)
	}
}

// The user's real LAN result that started the "only flag what you notice"
// rework: 0.89 ms idle, 50.9 ms while saturated, 1.03 ms jitter. Nobody hears
// that, so it must not be called a problem.
func TestUnnoticeableLANIsGood(t *testing.T) {
	r := &Result{LocalHost: "A", RemoteHost: "B", LocalKind: "wired", RemoteKind: "wired", LocalLinkMbps: 1000, RemoteLinkMbps: 1000,
		RemoteIP: "192.168.1.20", Route: "local", DownMbps: 930, UpMbps: 925, IdleRTT: 0.89, JitterMs: 1.03,
		DownLoadedRTT: 50.9, UpLoadedRTT: 30, LatencyMethod: "UDP", Sent: 300, UDPRateMbps: 100,
		UDPDown: &UDPStats{Sent: 83333, Received: 83333, JitterMs: 0.3},
		UDPUp:   &UDPStats{Sent: 83333, Received: 83333, JitterMs: 0.4},
		MTU:     &MTUResult{PathMTU: 1500, DFSupported: true, FragTested: true, FragmentsOK: true}}
	r.Grade("auto")
	if r.Verdict != "good" || len(r.Fixes) != 0 {
		t.Fatalf("verdict %s fixes %d\n%s", r.Verdict, len(r.Fixes), r.Text())
	}
	t.Log("\n" + r.Text())
}

// Loss you would hear in radio audio must be called out loudly.
func TestAudibleLossIsPoor(t *testing.T) {
	r := &Result{LocalKind: "wired", RemoteKind: "wired", RemoteIP: "100.92.218.56", IdleRTT: 30, JitterMs: 2,
		DownMbps: 40, UpMbps: 20, LatencyMethod: "UDP", Sent: 300, UDPRateMbps: 10,
		UDPDown: &UDPStats{Sent: 8333, Received: 8250, LossPct: 1.0, JitterMs: 3}}
	r.Grade("auto")
	if r.Verdict != "poor" || r.Notes[0].Level != "poor" || !strings.Contains(r.Notes[0].Text, "You will hear it") {
		t.Fatalf("%s\n%s", r.Verdict, r.Text())
	}
}
