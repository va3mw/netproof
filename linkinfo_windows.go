//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type winAdapter struct {
	Name                 string
	InterfaceDescription string
	ReceiveLinkSpeed     uint64
	PhysicalMediaType    string
	FullDuplex           *bool
	Virtual              bool
}

func osLinkInfo() map[string]osIfInfo {
	out := map[string]osIfInfo{}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-NetAdapter | Select-Object Name,InterfaceDescription,ReceiveLinkSpeed,PhysicalMediaType,FullDuplex,Virtual | ConvertTo-Json -Compress")
	hideWindow(cmd)
	b, err := cmd.Output()
	if err != nil {
		return out
	}
	s := strings.TrimSpace(string(b))
	var list []winAdapter
	if strings.HasPrefix(s, "{") {
		var one winAdapter
		if json.Unmarshal([]byte(s), &one) == nil {
			list = append(list, one)
		}
	} else {
		_ = json.Unmarshal([]byte(s), &list)
	}
	for _, ad := range list {
		oi := osIfInfo{Desc: ad.InterfaceDescription, Virtual: ad.Virtual}
		if ad.ReceiveLinkSpeed > 0 {
			oi.SpeedMbps = int(ad.ReceiveLinkSpeed / 1000000)
		}
		mt := strings.ToLower(ad.PhysicalMediaType)
		if strings.Contains(mt, "802.11") || strings.Contains(mt, "wireless") {
			oi.Wireless = true
		}
		if ad.FullDuplex != nil && !*ad.FullDuplex && !oi.Wireless {
			oi.Duplex = "half"
		}
		if strings.HasPrefix(strings.ToLower(ad.Name), "vethernet") || strings.Contains(strings.ToLower(ad.InterfaceDescription), "hyper-v") {
			oi.Virtual = true
		}
		out[ad.Name] = oi
	}
	return out
}

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
