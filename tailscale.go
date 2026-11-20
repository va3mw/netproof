package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

var tailscaleNet6 = mustCIDR("fd7a:115c:a1e0::/48")

// isTailscaleIP: Tailscale hands out 100.64.0.0/10 and fd7a:115c:a1e0::/48.
func isTailscaleIP(ip net.IP) bool {
	return ip != nil && (tailscaleNet.Contains(ip) || tailscaleNet6.Contains(ip))
}

type tsPeer struct {
	HostName string // "boathouse2"
	DNSName  string // "boathouse2.tail1234.ts.net" (MagicDNS)
	IPv4     string
	IPv6     string
	Online   bool
}

// tailscalePeers reads the tailnet's machines from the local Tailscale
// service (named pipe on Windows, the tailscale CLI elsewhere).
func tailscalePeers(ctx context.Context) ([]tsPeer, error) {
	if !tailscaleInstalled() {
		return nil, fmt.Errorf("tailscale is not installed")
	}
	out, err := tailscaleStatusJSON(ctx)
	if err != nil {
		return nil, err
	}
	var st struct {
		Peer map[string]struct {
			HostName     string
			DNSName      string
			TailscaleIPs []string
			Online       bool
		}
	}
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, err
	}
	var peers []tsPeer
	for _, p := range st.Peer {
		tp := tsPeer{HostName: p.HostName, DNSName: strings.TrimSuffix(p.DNSName, "."), Online: p.Online}
		for _, x := range p.TailscaleIPs {
			if v := net.ParseIP(x); v != nil {
				if v.To4() != nil {
					tp.IPv4 = x
				} else {
					tp.IPv6 = x
				}
			}
		}
		peers = append(peers, tp)
	}
	return peers, nil
}

// shortName of "boathouse2.tail1234.ts.net" is "boathouse2".
func shortName(dns string) string {
	if i := strings.IndexByte(dns, '.'); i > 0 {
		return dns[:i]
	}
	return dns
}

// tailscaleLookup finds a machine by MagicDNS name (short or full) or by
// Tailscale host name.
func tailscaleLookup(ctx context.Context, name string) (ip, fqdn string, ok bool) {
	peers, err := tailscalePeers(ctx)
	if err != nil {
		return "", "", false
	}
	n := strings.ToLower(strings.TrimSuffix(name, "."))
	for _, p := range peers {
		dns := strings.ToLower(p.DNSName)
		if n == dns || n == shortName(dns) || n == strings.ToLower(p.HostName) {
			ip = p.IPv4
			if ip == "" {
				ip = p.IPv6
			}
			return ip, p.DNSName, ip != ""
		}
	}
	return "", "", false
}

// resolveTarget turns what the user typed into one IP address, used for
// every part of the test. IPv4 is preferred so TCP, UDP and the MTU test all
// take the same path. Tailscale names are looked up in Tailscale itself when
// the computer's DNS does not know them (MagicDNS off, or Linux without
// Tailscale DNS).
func resolveTarget(ctx context.Context, host string) (ip, name string, err error) {
	if v := net.ParseIP(host); v != nil {
		return host, "", nil
	}
	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1", "", nil
	}
	lctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	addrs, derr := net.DefaultResolver.LookupIPAddr(lctx, host)
	cancel()
	if derr == nil && len(addrs) > 0 {
		pick := addrs[0].IP
		for _, a := range addrs {
			if a.IP.To4() != nil {
				pick = a.IP
				break
			}
		}
		return pick.String(), host, nil
	}
	tctx, tcancel := context.WithTimeout(ctx, 8*time.Second)
	defer tcancel()
	if ip, fqdn, ok := tailscaleLookup(tctx, host); ok {
		return ip, fqdn, nil
	}
	return "", "", fmt.Errorf("could not find a computer called %q. Check the spelling, or type its IP address. For a Tailscale name, make sure Tailscale is running and connected on both computers (MagicDNS names look like \"boathouse2\" or \"boathouse2.tail1234.ts.net\")", host)
}
