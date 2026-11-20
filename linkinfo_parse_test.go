package main

import "testing"

func TestParseMacMedia(t *testing.T) {
	cases := []struct {
		in     string
		speed  int
		duplex string
	}{
		{"en0: flags=8863<UP>\n\tmedia: autoselect (1000baseT <full-duplex,flow-control>)\n\tstatus: active", 1000, "full"},
		{"\tmedia: autoselect (100baseTX <half-duplex>)", 100, "half"},
		{"\tmedia: autoselect (10GbaseT <full-duplex>)", 10000, "full"},
		{"\tmedia: autoselect (2500Base-T <full-duplex>)", 2500, "full"},
		{"\tmedia: autoselect\n\tstatus: active", 0, ""},
		{"\tmedia: autoselect (none)", 0, ""},
	}
	for _, c := range cases {
		s, d := parseMacMedia(c.in)
		if s != c.speed || d != c.duplex {
			t.Errorf("%q -> %d %q, want %d %q", c.in, s, d, c.speed, c.duplex)
		}
	}
}

func TestParseMacHardwarePorts(t *testing.T) {
	out := `
Hardware Port: Ethernet
Device: en0
Ethernet Address: 00:11:22:33:44:55

Hardware Port: Wi-Fi
Device: en1
Ethernet Address: 66:77:88:99:aa:bb

Hardware Port: Thunderbolt Bridge
Device: bridge0
Ethernet Address: N/A

VLAN Configurations
===================
`
	p := parseMacHardwarePorts(out)
	if p["en0"] != "Ethernet" || p["en1"] != "Wi-Fi" || p["bridge0"] != "Thunderbolt Bridge" || len(p) != 3 {
		t.Fatalf("%v", p)
	}
	if !isMacWiFiPort(p["en1"]) || isMacWiFiPort(p["en0"]) {
		t.Fatal("Wi-Fi detection wrong")
	}
}
