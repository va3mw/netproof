//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func osLinkInfo() map[string]osIfInfo {
	out := map[string]osIfInfo{}
	ents, _ := os.ReadDir("/sys/class/net")
	for _, e := range ents {
		name := e.Name()
		base := filepath.Join("/sys/class/net", name)
		var oi osIfInfo
		if b, err := os.ReadFile(filepath.Join(base, "speed")); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && n > 0 && n < 1000000 {
				oi.SpeedMbps = n
			}
		}
		if b, err := os.ReadFile(filepath.Join(base, "duplex")); err == nil {
			oi.Duplex = strings.TrimSpace(string(b))
		}
		if _, err := os.Stat(filepath.Join(base, "wireless")); err == nil {
			oi.Wireless = true
		} else if _, err := os.Stat(filepath.Join(base, "phy80211")); err == nil {
			oi.Wireless = true
		}
		if _, err := os.Stat(filepath.Join(base, "device")); err != nil {
			oi.Virtual = true // no physical device behind it
		}
		out[name] = oi
	}
	return out
}

func hideWindow(cmd *exec.Cmd) {}
