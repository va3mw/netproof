package main

// Wi-Fi advice that fits the computers actually on Wi-Fi: their names, their
// operating system and their adapter, instead of every OS's steps at once.

import "fmt"

type wifiSide struct {
	host, os, iface string
}

// wifiSides lists the ends of the test that are on Wi-Fi.
func wifiSides(r *Result) []wifiSide {
	var out []wifiSide
	if r.LocalKind == "wifi" {
		out = append(out, wifiSide{r.LocalHost, r.LocalOS, r.LocalIface})
	}
	if r.RemoteKind == "wifi" {
		out = append(out, wifiSide{r.RemoteHost, r.RemoteOS, r.RemoteIface})
	}
	return out
}

var osNames = map[string]string{"windows": "Windows", "linux": "Linux", "darwin": "Mac"}

func (w wifiSide) label() string {
	if n, ok := osNames[w.os]; ok {
		return fmt.Sprintf("On %s (%s)", w.host, n)
	}
	return "On " + w.host
}

// wifiSteps is the Wi-Fi checklist, most effective first.
func wifiSteps(sides []wifiSide) []string {
	cable := "For the radio or SDR computer, use a network cable: it is the only fix that removes the problem. Wi-Fi takes turns on a shared channel, so delay and loss come and go however good the signal is. If you cannot run a cable, MoCA adapters over existing TV coax come close to a cable; powerline adapters are hit and miss."
	switch len(sides) {
	case 1:
		cable = sides[0].host + " is on Wi-Fi. " + cable
	case 2:
		cable = "Both " + sides[0].host + " and " + sides[1].host + " are on Wi-Fi. " + cable
	}
	steps := []string{cable}

	known := len(sides) > 0
	for _, s := range sides {
		if _, ok := osNames[s.os]; !ok {
			known = false
		}
	}
	if !known {
		// Results from copies that do not say their operating system.
		steps = append(steps,
			"Turn off Wi-Fi power saving, which makes the adapter doze between packets. On Windows: Power Options > Change advanced power settings > Wireless Adapter Settings > Power Saving Mode > Maximum Performance, and set \"MIMO Power Save Mode\" to \"No SMPS\" in the adapter's Advanced settings if it is there. On Linux: sudo iw dev <wifi-name> set power_save off.",
			"Stop the computer looking for other networks while you operate: each scan pauses traffic, which shows as regular lag spikes and audio clicks. On Windows, close the Wi-Fi network list and set \"Roaming Aggressiveness\" to \"Lowest\" in the adapter's Advanced settings.",
			"Update the Wi-Fi driver; many lag-spike problems are driver bugs.",
		)
	} else {
		for _, s := range sides {
			steps = append(steps, osWifiSteps(s)...)
		}
	}

	return append(steps,
		"Find out what else is on the Wi-Fi. Every device on the same channel takes turns with yours, so a TV streaming, a cloud backup, phones uploading photos or one old slow device makes everyone wait. Move busy devices to a cable or to the other band.",
		"Give the 5 GHz (or 6 GHz) band its own network name and connect the radio computer only to that, so it cannot hop between bands or mesh units in the middle of a session.",
		"In the router or access point: make sure WMM is on (Wi-Fi traffic priority needs it), choose a fixed channel instead of Auto, avoid DFS channels (52 to 144 on 5 GHz: when the router detects radar it must stop and move, which drops everyone for a minute or more), and use 80 MHz channel width rather than 160 MHz.",
		"With a mesh system, connect the mesh units to each other with cables (wired backhaul). Every hop over the air adds delay and jitter and cuts the speed.",
		"Moving closer to the access point only helps when the signal is weak (one or two bars, or worse than -67 dBm). With a good signal it changes nothing, and the steps above matter more.",
	)
}

// osWifiSteps: power saving, background scans and drivers, for one computer.
func osWifiSteps(s wifiSide) []string {
	on := s.label()
	switch s.os {
	case "windows":
		return []string{
			on + ", turn off Wi-Fi power saving, which makes the adapter doze between packets: Control Panel > Power Options > Change plan settings > Change advanced power settings > Wireless Adapter Settings > Power Saving Mode > Maximum Performance, and keep a laptop plugged in. In the adapter's Advanced settings (Device Manager), set \"MIMO Power Save Mode\" to \"No SMPS\" if it is there.",
			on + ", stop it looking for other networks while you operate. Windows scans now and then, often about once a minute, and each scan pauses traffic: that shows as regular lag spikes and audio clicks. Close the Wi-Fi network list, and for a computer that stays put, set \"Roaming Aggressiveness\" to \"Lowest\" and, on Intel adapters, \"Global BG Scan Blocking\" to \"Always\" in the adapter's Advanced settings.",
			on + ", update the Wi-Fi driver from the adapter maker (Intel, Realtek, MediaTek, Qualcomm), not only Windows Update. Many lag-spike problems are driver bugs.",
		}
	case "linux":
		dev := s.iface
		if dev == "" {
			dev = "<wifi-name>" // iw dev lists it
		}
		return []string{
			on + ", turn off Wi-Fi power saving, which makes the adapter doze between packets: sudo iw dev " + dev + " set power_save off (iw dev " + dev + " get power_save shows the setting). That lasts until a restart. To keep it with NetworkManager, create /etc/NetworkManager/conf.d/wifi-powersave.conf with the two lines [connection] and wifi.powersave = 2, then run sudo systemctl restart NetworkManager.",
			on + ", keep it on one access point. NetworkManager looks for other access points now and then and can move to another band or mesh unit mid-session. For a computer that stays put, lock the connection to your access point: nmcli -f SSID,BSSID,CHAN,SIGNAL device wifi list shows its BSSID; then nmcli connection modify \"<connection name>\" 802-11-wireless.bssid <BSSID> and reconnect. Close the network menu while operating.",
			on + ", keep the kernel and the Wi-Fi firmware up to date (sudo apt update && sudo apt full-upgrade). Many Wi-Fi lag problems are driver or firmware bugs.",
		}
	case "darwin":
		return []string{
			on + ", turn off AirDrop and Handoff while operating. They share the Wi-Fi radio (the awdl0 link) and cause regular lag spikes; sudo ifconfig awdl0 down stops it until the next restart. Keep a laptop plugged in.",
			on + ", keep macOS up to date: its Wi-Fi drivers come with it.",
		}
	}
	return nil
}
