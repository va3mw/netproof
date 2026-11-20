//go:build darwin

package main

import (
	"context"
	"os/exec"
	"time"
)

func run(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, name, args...).Output()
	return string(out)
}

// osLinkInfo on macOS: which device is Wi-Fi comes from networksetup, and
// link speed and duplex from ifconfig's "media:" line.
func osLinkInfo() map[string]osIfInfo {
	out := map[string]osIfInfo{}
	ports := parseMacHardwarePorts(run("/usr/sbin/networksetup", "-listallhardwareports"))
	for dev, port := range ports {
		oi := osIfInfo{Desc: port}
		if isMacWiFiPort(port) {
			oi.Wireless = true
		} else {
			oi.SpeedMbps, oi.Duplex = parseMacMedia(run("/sbin/ifconfig", dev))
		}
		out[dev] = oi
	}
	return out
}
