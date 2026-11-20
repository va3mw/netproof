//go:build windows

package main

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ifTypeIEEE80211 = 71  // Wi-Fi
	ifTypeTunnel    = 131 // tunnel encapsulation
	ifTypePropVirt  = 53  // proprietary virtual (Wintun, TAP, ...)
)

// osLinkInfo reads adapter type and link speed straight from the Windows IP
// Helper API (GetAdaptersAddresses). No PowerShell, no hidden windows.
func osLinkInfo() map[string]osIfInfo {
	out := map[string]osIfInfo{}
	size := uint32(16 * 1024)
	var buf []byte
	for i := 0; i < 3; i++ {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if err != windows.ERROR_BUFFER_OVERFLOW {
			return out
		}
		buf = nil
	}
	if buf == nil {
		return out
	}
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); aa != nil; aa = aa.Next {
		name := windows.UTF16PtrToString(aa.FriendlyName)
		desc := windows.UTF16PtrToString(aa.Description)
		oi := osIfInfo{Desc: desc}
		if s := aa.ReceiveLinkSpeed; s > 0 && s != ^uint64(0) {
			oi.SpeedMbps = int(s / 1000000)
		}
		ld := strings.ToLower(desc)
		switch {
		case aa.IfType == ifTypeIEEE80211:
			oi.Wireless = true
		case aa.IfType == ifTypeTunnel || aa.IfType == ifTypePropVirt:
			if !isVPNName(desc) && !isVPNName(name) {
				oi.Virtual = true
			}
		}
		if strings.Contains(ld, "hyper-v") || strings.Contains(ld, "virtualbox") || strings.Contains(ld, "vmware") ||
			strings.HasPrefix(strings.ToLower(name), "vethernet") {
			oi.Virtual = true
		}
		out[name] = oi
	}
	return out
}
