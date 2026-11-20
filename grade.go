package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Profile is a hard coded "what good looks like" for one kind of connection.
type Profile struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	ExpectMbps float64 `json:"expectMbps"` // typical real TCP throughput, 0 = no fixed expectation
	ThrGood    float64 `json:"thrGood"`
	ThrFair    float64 `json:"thrFair"`
	RTTGood    float64 `json:"rttGood"`
	RTTFair    float64 `json:"rttFair"`
	JitGood    float64 `json:"jitGood"`
	JitFair    float64 `json:"jitFair"`
	LossGood   float64 `json:"lossGood"`
	LossFair   float64 `json:"lossFair"`
	BloatGood  float64 `json:"bloatGood"` // extra delay under load, ms
	BloatFair  float64 `json:"bloatFair"`
}

// A healthy wired gigabit link moves about 940 Mb/s of real TCP data (the
// rest is Ethernet, IP and TCP overhead). Fast Ethernet: about 94 Mb/s.
var profiles = map[string]Profile{
	"1g": {Key: "1g", Name: "1 Gb/s wired LAN", ExpectMbps: 940, ThrGood: 850, ThrFair: 600,
		RTTGood: 1, RTTFair: 3, JitGood: 0.5, JitFair: 2, LossGood: 0.01, LossFair: 0.5, BloatGood: 10, BloatFair: 40},
	"100m": {Key: "100m", Name: "100 Mb/s wired LAN", ExpectMbps: 94, ThrGood: 85, ThrFair: 60,
		RTTGood: 1, RTTFair: 3, JitGood: 0.5, JitFair: 2, LossGood: 0.01, LossFair: 0.5, BloatGood: 20, BloatFair: 60},
	"wifi": {Key: "wifi", Name: "Wi-Fi", ExpectMbps: 0, ThrGood: 300, ThrFair: 100,
		RTTGood: 4, RTTFair: 15, JitGood: 2, JitFair: 6, LossGood: 0.1, LossFair: 1, BloatGood: 30, BloatFair: 100},
	"vpn": {Key: "vpn", Name: "VPN / Tailscale / Internet", ExpectMbps: 0, ThrGood: 0, ThrFair: 0,
		RTTGood: 30, RTTFair: 80, JitGood: 3, JitFair: 10, LossGood: 0.1, LossFair: 1, BloatGood: 30, BloatFair: 100},
}

var profileOrder = []string{"1g", "100m", "wifi", "vpn"}

type Metric struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Grade  string  `json:"grade"` // good, fair, poor, info
	Expect string  `json:"expect"`
}

type Result struct {
	Time    string `json:"time"`
	Version string `json:"version"`

	LocalHost     string `json:"localHost"`
	LocalIP       string `json:"localIP"`
	LocalIface    string `json:"localIface"`
	LocalKind     string `json:"localKind"`
	LocalLinkMbps int    `json:"localLinkMbps"`
	LocalDuplex   string `json:"localDuplex,omitempty"`

	RemoteHost     string `json:"remoteHost"`
	RemoteIP       string `json:"remoteIP"`
	RemoteOS       string `json:"remoteOS"`
	RemoteIface    string `json:"remoteIface"`
	RemoteKind     string `json:"remoteKind"`
	RemoteLinkMbps int    `json:"remoteLinkMbps"`
	RemoteDuplex   string `json:"remoteDuplex,omitempty"`

	DownMbps      float64 `json:"downMbps"`
	UpMbps        float64 `json:"upMbps"`
	IdleRTT       float64 `json:"idleRtt"`
	IdleRTTMin    float64 `json:"idleRttMin"`
	IdleRTTMax    float64 `json:"idleRttMax"`
	JitterMs      float64 `json:"jitterMs"`
	Sent          int     `json:"sent"`
	Lost          int     `json:"lost"`
	LossPct       float64 `json:"lossPct"`
	DownLoadedRTT float64 `json:"downLoadedRtt"`
	UpLoadedRTT   float64 `json:"upLoadedRtt"`
	LatencyMethod string  `json:"latencyMethod"`
	Practice      bool    `json:"practice"`

	ProfileKey  string   `json:"profileKey"`
	ProfileName string   `json:"profileName"`
	ProfileAuto bool     `json:"profileAuto"`
	Metrics     []Metric `json:"metrics"`
	Verdict     string   `json:"verdict"`
	Headline    string   `json:"headline"`
	Findings    []string `json:"findings"`
}

// autoProfile guesses what the connection is supposed to be.
func (r *Result) autoProfile() string {
	ip := r.RemoteIP
	if r.LocalKind == "vpn" || r.RemoteKind == "vpn" || (ip != "" && tailscaleNet.Contains(parseIP(ip))) {
		return "vpn"
	}
	if r.LocalKind == "wifi" || r.RemoteKind == "wifi" {
		return "wifi"
	}
	link := minNonZero(r.LocalLinkMbps, r.RemoteLinkMbps)
	if link > 0 && link < 1000 {
		return "100m"
	}
	return "1g"
}

func minNonZero(a, b int) int {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	case a < b:
		return a
	}
	return b
}

func grade3(v, good, fair float64, higherBetter bool) string {
	if higherBetter {
		switch {
		case v >= good:
			return "good"
		case v >= fair:
			return "fair"
		}
		return "poor"
	}
	switch {
	case v <= good:
		return "good"
	case v <= fair:
		return "fair"
	}
	return "poor"
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
func ms(v float64) string {
	if v < 10 {
		return f2(v) + " ms"
	}
	return f1(v) + " ms"
}
func itoa(i int) string            { return strconv.Itoa(i) }
func ftoa(v float64, p int) string { return strconv.FormatFloat(v, 'f', p, 64) }
func mbps(v float64) string {
	if v >= 100 {
		return strconv.FormatFloat(v, 'f', 0, 64) + " Mb/s"
	}
	return f1(v) + " Mb/s"
}

// Grade scores the result against a profile ("auto" picks one) and writes the
// plain English verdict.
func (r *Result) Grade(profile string) {
	r.ProfileAuto = profile == "" || profile == "auto"
	if r.ProfileAuto {
		profile = r.autoProfile()
	}
	p := profiles[profile]
	r.ProfileKey, r.ProfileName = p.Key, p.Name
	r.Metrics, r.Findings = nil, nil
	if r.Practice {
		r.gradePractice()
		return
	}

	down := r.RemoteHost + " → " + r.LocalHost
	up := r.LocalHost + " → " + r.RemoteHost
	thrExpect := "no fixed target"
	if p.ExpectMbps > 0 {
		thrExpect = "about " + mbps(p.ExpectMbps) + " (at least " + mbps(p.ThrGood) + ")"
	} else if p.ThrGood > 0 {
		thrExpect = "good Wi-Fi: " + mbps(p.ThrGood) + " or more"
	}
	thrGrade := func(v float64) string {
		if p.ThrGood == 0 {
			return "info"
		}
		return grade3(v, p.ThrGood, p.ThrFair, true)
	}
	bloat := maxf(r.DownLoadedRTT, r.UpLoadedRTT) - r.IdleRTT
	if bloat < 0 {
		bloat = 0
	}

	r.Metrics = []Metric{
		{Key: "down", Label: "Download  " + down, Value: r.DownMbps, Unit: "Mb/s", Grade: thrGrade(r.DownMbps), Expect: thrExpect},
		{Key: "up", Label: "Upload  " + up, Value: r.UpMbps, Unit: "Mb/s", Grade: thrGrade(r.UpMbps), Expect: thrExpect},
		{Key: "rtt", Label: "Delay (round trip)", Value: r.IdleRTT, Unit: "ms", Grade: grade3(r.IdleRTT, p.RTTGood, p.RTTFair, false), Expect: "under " + ms(p.RTTGood)},
		{Key: "jitter", Label: "Jitter", Value: r.JitterMs, Unit: "ms", Grade: grade3(r.JitterMs, p.JitGood, p.JitFair, false), Expect: "under " + ms(p.JitGood)},
	}
	if r.LatencyMethod == "UDP" {
		r.Metrics = append(r.Metrics, Metric{Key: "loss", Label: "Packet loss", Value: r.LossPct, Unit: "%", Grade: grade3(r.LossPct, p.LossGood, p.LossFair, false), Expect: "0 %"})
	} else {
		r.Metrics = append(r.Metrics, Metric{Key: "loss", Label: "Packet loss", Value: 0, Unit: "%", Grade: "info", Expect: "not measured (UDP blocked)"})
	}
	r.Metrics = append(r.Metrics, Metric{Key: "bloat", Label: "Extra delay when busy", Value: bloat, Unit: "ms", Grade: grade3(bloat, p.BloatGood, p.BloatFair, false), Expect: "under " + ms(p.BloatGood)})

	worst := "good"
	rank := map[string]int{"info": 0, "good": 0, "fair": 1, "poor": 2}
	for _, m := range r.Metrics {
		if rank[m.Grade] > rank[worst] {
			worst = m.Grade
		}
	}

	// ---- plain English findings, worst first
	var poor, fair, notes []string
	add := func(g, s string) {
		switch g {
		case "poor":
			poor = append(poor, s)
		case "fair":
			fair = append(fair, s)
		default:
			notes = append(notes, s)
		}
	}

	for _, side := range []struct {
		host, iface, kind, duplex string
		link                      int
	}{
		{r.LocalHost, r.LocalIface, r.LocalKind, r.LocalDuplex, r.LocalLinkMbps},
		{r.RemoteHost, r.RemoteIface, r.RemoteKind, r.RemoteDuplex, r.RemoteLinkMbps},
	} {
		if side.kind == "wired" && side.link > 0 && side.link <= 10 {
			add("poor", fmt.Sprintf("%s's network adapter (%s) is connected at only %d Mb/s. That is ancient and almost always means a broken cable or a damaged port.", side.host, side.iface, side.link))
			worst = "poor"
		} else if side.kind == "wired" && side.link == 100 && p.Key != "vpn" {
			add("poor", fmt.Sprintf("%s's network adapter (%s) is connected at only 100 Mb/s instead of 1 Gb/s. If this network is meant to be gigabit, the usual causes are a bad or cheap cable (gigabit needs all 8 wires, Cat5e or better), a bad wall jack, an old switch, or an adapter setting.", side.host, side.iface))
			if p.Key == "1g" {
				worst = "poor"
			}
		}
		if side.duplex == "half" && side.kind == "wired" {
			add("poor", fmt.Sprintf("%s's network adapter is running in half duplex. That causes collisions and terrible speeds. Set the adapter and the switch port to Auto.", side.host))
			worst = "poor"
		}
	}

	for _, m := range r.Metrics[:2] {
		if m.Grade == "poor" || m.Grade == "fair" {
			dir := "from " + r.RemoteHost + " to " + r.LocalHost
			if m.Key == "up" {
				dir = "from " + r.LocalHost + " to " + r.RemoteHost
			}
			s := fmt.Sprintf("Speed %s was %s.", dir, mbps(m.Value))
			if p.ExpectMbps > 0 {
				s += fmt.Sprintf(" A healthy %s should move about %s (%d%% of that is what you got).", p.Name, mbps(p.ExpectMbps), int(m.Value*100/p.ExpectMbps+0.5))
			} else {
				s += fmt.Sprintf(" Good %s usually does %s or more.", p.Name, mbps(p.ThrGood))
			}
			add(m.Grade, s)
		}
	}
	if r.LocalKind == "wired" && r.RemoteKind == "wired" && r.DownMbps > 0 && r.UpMbps > 0 {
		lo, hi := minf(r.DownMbps, r.UpMbps), maxf(r.DownMbps, r.UpMbps)
		if lo < hi*0.6 && p.ThrGood > 0 {
			add("fair", fmt.Sprintf("Speed is lopsided: %s one way and %s the other. On a wired network both directions should be about the same. Suspect a cable, a switch port, a duplex mismatch, or a slow computer at one end.", mbps(hi), mbps(lo)))
		}
	}
	if g := gradeOf(r.Metrics, "rtt"); g != "good" {
		add(g, fmt.Sprintf("Delay is %s round trip (expected under %s for %s). Something between the two computers is slow or overloaded.", ms(r.IdleRTT), ms(p.RTTGood), p.Name))
	}
	if g := gradeOf(r.Metrics, "jitter"); g != "good" {
		add(g, fmt.Sprintf("Jitter is %s (expected under %s). The delay keeps changing, which makes live audio, video calls and remote control stutter or drop out.", ms(r.JitterMs), ms(p.JitGood)))
	}
	if r.LatencyMethod == "UDP" && r.Lost > 0 {
		g := gradeOf(r.Metrics, "loss")
		s := fmt.Sprintf("%d of %d test packets were lost (%s%%).", r.Lost, r.Sent, f2(r.LossPct))
		if p.Key == "1g" || p.Key == "100m" {
			s += " On a wired LAN this should be zero. Lost packets point to bad cables, a failing switch or an overloaded link."
		} else {
			s += " Lost packets cause dropouts in live audio and video."
		}
		add(g, s)
	}
	if g := gradeOf(r.Metrics, "bloat"); g != "good" {
		add(g, fmt.Sprintf("When the network was busy, delay jumped from %s to %s. This is called bufferbloat: everything gets laggy while someone is downloading or uploading.", ms(r.IdleRTT), ms(maxf(r.DownLoadedRTT, r.UpLoadedRTT))))
	}
	if r.LatencyMethod != "UDP" {
		add("info", "UDP packets were blocked (probably by a firewall), so delay was measured over TCP and packet loss could not be measured. Allow NetProof through the firewall on both computers for a complete test.")
	}
	if p.Key == "vpn" {
		add("info", "This test ran over a VPN. VPN speed depends on both internet connections and on how the VPN routes traffic, so there is no fixed speed target. Delay, jitter and loss matter most here.")
		if r.IdleRTT > 0 && p.Key == "vpn" && tailscaleNet.Contains(parseIP(r.RemoteIP)) && r.IdleRTT > 40 {
			add("info", "If Tailscale is relaying this connection (\"relay\" in tailscale status), delay will be higher than a direct connection.")
		}
	}

	r.Findings = append(append(poor, fair...), notes...)
	r.Verdict = worst
	switch worst {
	case "good":
		r.Headline = "Your network looks healthy"
		r.Findings = append([]string{"Everything measured is where it should be for a " + p.Name + "."}, r.Findings...)
	case "fair":
		r.Headline = "Your network works, but it is not as good as it should be"
	default:
		r.Headline = "Your network has problems"
	}
}

// gradePractice: a loopback test never touches the network, so the numbers
// only show how NetProof works. Nothing is graded.
func (r *Result) gradePractice() {
	r.ProfileKey, r.ProfileName, r.ProfileAuto = "practice", "Practice run (no network)", true
	exp := "practice run: not graded"
	r.Metrics = []Metric{
		{Key: "down", Label: "Download (practice)", Value: r.DownMbps, Unit: "Mb/s", Grade: "info", Expect: exp},
		{Key: "up", Label: "Upload (practice)", Value: r.UpMbps, Unit: "Mb/s", Grade: "info", Expect: exp},
		{Key: "rtt", Label: "Delay (round trip)", Value: r.IdleRTT, Unit: "ms", Grade: "info", Expect: exp},
		{Key: "jitter", Label: "Jitter", Value: r.JitterMs, Unit: "ms", Grade: "info", Expect: exp},
		{Key: "loss", Label: "Packet loss", Value: r.LossPct, Unit: "%", Grade: "info", Expect: exp},
		{Key: "bloat", Label: "Extra delay when busy", Value: maxf(0, maxf(r.DownLoadedRTT, r.UpLoadedRTT)-r.IdleRTT), Unit: "ms", Grade: "info", Expect: exp},
	}
	r.Verdict = "practice"
	r.Headline = "Practice run finished"
	r.Findings = []string{
		"This test never left this computer, so the speeds are far higher than any real network and nothing is graded. It only shows you what a test looks like.",
		"For a real test, start NetProof on a second computer and pick it from the list (or type its address).",
	}
}

func gradeOf(ms []Metric, key string) string {
	for _, m := range ms {
		if m.Key == key {
			return m.Grade
		}
	}
	return "info"
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Text is a plain text report for the console or for pasting into an email.
func (r *Result) Text() string {
	var b strings.Builder
	mark := map[string]string{"good": "OK  ", "fair": "FAIR", "poor": "BAD ", "info": "    "}
	fmt.Fprintf(&b, "NetProof %s report  %s\n", r.Version, r.Time)
	fmt.Fprintf(&b, "%s (%s) <-> %s (%s)\n", r.LocalHost, nz(r.LocalIP), r.RemoteHost, r.RemoteIP)
	fmt.Fprintf(&b, "This end:  %s\n", sideDesc(r.LocalIface, r.LocalKind, r.LocalLinkMbps))
	fmt.Fprintf(&b, "Far end:   %s\n", sideDesc(r.RemoteIface, r.RemoteKind, r.RemoteLinkMbps))
	auto := ""
	if r.ProfileAuto {
		auto = " (auto detected)"
	}
	fmt.Fprintf(&b, "Compared against: %s%s\n\n", r.ProfileName, auto)
	fmt.Fprintf(&b, ">>> %s <<<\n\n", strings.ToUpper(r.Headline))
	for _, m := range r.Metrics {
		v := f2(m.Value) + " " + m.Unit
		if m.Unit == "Mb/s" {
			v = mbps(m.Value)
		}
		fmt.Fprintf(&b, "  [%s] %-34s %14s   expected: %s\n", mark[m.Grade], m.Label, v, m.Expect)
	}
	fmt.Fprintf(&b, "\n  Delay range %s to %s, %d probes over %s, busy delay %s down / %s up\n\n",
		ms(r.IdleRTTMin), ms(r.IdleRTTMax), r.Sent, r.LatencyMethod, ms(r.DownLoadedRTT), ms(r.UpLoadedRTT))
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "  * %s\n", f)
	}
	return b.String()
}

func nz(s string) string {
	if s == "" {
		return "auto"
	}
	return s
}

func sideDesc(iface, kind string, link int) string {
	if iface == "" {
		return "unknown connection"
	}
	k := map[string]string{"wired": "wired", "wifi": "Wi-Fi", "vpn": "VPN", "virtual": "virtual"}[kind]
	s := iface + " (" + k
	if link > 0 && kind != "vpn" {
		s += ", link " + fmtLink(link)
	}
	return s + ")"
}
