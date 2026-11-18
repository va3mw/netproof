package main

import (
	"net"
	"sort"
	"strings"
	"time"
)

// IfaceInfo is one usable network connection on this computer.
type IfaceInfo struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Kind     string `json:"kind"` // wired, wifi, vpn, virtual
	LinkMbps int    `json:"linkMbps"`
	Duplex   string `json:"duplex,omitempty"`
	Desc     string `json:"desc,omitempty"`
	Label    string `json:"label"`
}

// osIfInfo is what the OS-specific code can tell us about an adapter.
type osIfInfo struct {
	SpeedMbps int
	Wireless  bool
	Duplex    string
	Desc      string
	Virtual   bool
}

var tailscaleNet = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func isVPNName(s string) bool {
	s = strings.ToLower(s)
	for _, k := range []string{"tun", "tap", "wg", "ppp", "utun", "ipsec", "nordlynx"} {
		if strings.HasPrefix(s, k) {
			return true
		}
	}
	for _, k := range []string{"tailscale", "wireguard", "zerotier", "openvpn", "vpn", "anyconnect", "fortinet", "globalprotect", "tunnel"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func isSkippedName(s string) bool {
	s = strings.ToLower(s)
	for _, k := range []string{"docker", "veth", "br-", "virbr", "lxc", "lxd", "cni", "flannel", "vmnet", "awdl", "llw", "isatap", "teredo"} {
		if strings.HasPrefix(s, k) {
			return true
		}
	}
	return false
}

// interfaces lists up, non-loopback IPv4 connections, cached for a few seconds
// because asking Windows about link speed is slow.
func (a *App) interfaces() []IfaceInfo {
	a.mu.Lock()
	if a.ifCache != nil && time.Since(a.ifCacheAt) < 15*time.Second {
		c := a.ifCache
		a.mu.Unlock()
		return c
	}
	a.mu.Unlock()

	info := osLinkInfo()
	var out []IfaceInfo
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || isSkippedName(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			oi := info[ifc.Name]
			it := IfaceInfo{Name: ifc.Name, IP: ipn.IP.String(), LinkMbps: oi.SpeedMbps, Duplex: oi.Duplex, Desc: oi.Desc}
			switch {
			case tailscaleNet.Contains(ipn.IP) || isVPNName(ifc.Name) || isVPNName(oi.Desc) || ifc.Flags&net.FlagPointToPoint != 0:
				it.Kind = "vpn"
			case oi.Wireless:
				it.Kind = "wifi"
			case oi.Virtual:
				it.Kind = "virtual"
			default:
				it.Kind = "wired"
			}
			it.Label = ifaceLabel(it)
			out = append(out, it)
		}
	}
	rank := map[string]int{"wired": 0, "wifi": 1, "vpn": 2, "virtual": 3}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Kind] < rank[out[j].Kind] })

	a.mu.Lock()
	a.ifCache, a.ifCacheAt = out, time.Now()
	a.mu.Unlock()
	return out
}

func ifaceLabel(it IfaceInfo) string {
	kind := map[string]string{"wired": "Wired", "wifi": "Wi-Fi", "vpn": "VPN", "virtual": "Virtual"}[it.Kind]
	if it.Kind == "vpn" && (strings.Contains(strings.ToLower(it.Name+it.Desc), "tailscale") || tailscaleNet.Contains(net.ParseIP(it.IP))) {
		kind = "Tailscale VPN"
	}
	s := kind + ": " + it.Name + "  (" + it.IP + ")"
	if it.LinkMbps > 0 && it.Kind != "vpn" {
		s += "  " + fmtLink(it.LinkMbps)
	}
	return s
}

func fmtLink(mbps int) string {
	if mbps >= 1000 {
		g := float64(mbps) / 1000
		if g == float64(int(g)) {
			return itoa(int(g)) + " Gb/s"
		}
		return ftoa(g, 1) + " Gb/s"
	}
	return itoa(mbps) + " Mb/s"
}

func (a *App) ifaceForIP(ip string) (IfaceInfo, bool) {
	for _, it := range a.interfaces() {
		if it.IP == ip {
			return it, true
		}
	}
	return IfaceInfo{}, false
}

// broadcastAddrs returns the subnet broadcast address of each LAN.
func broadcastAddrs() []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil || len(ipn.Mask) != 4 {
				continue
			}
			ones, _ := ipn.Mask.Size()
			if ones >= 31 {
				continue
			}
			b := make(net.IP, 4)
			for i := range b {
				b[i] = ip4[i] | ^ipn.Mask[i]
			}
			out = append(out, b.String())
		}
	}
	return out
}

// localIPFor asks the OS which local address it would use to reach target.
func localIPFor(target string) string {
	c, err := net.Dial("udp4", net.JoinHostPort(target, "9"))
	if err != nil {
		return ""
	}
	defer c.Close()
	if ua, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return ua.IP.String()
	}
	return ""
}
