package main

// Parsing for macOS network tool output. Kept platform-neutral so it is
// tested on every platform.

import (
	"regexp"
	"strconv"
	"strings"
)

// "media: autoselect (1000baseT <full-duplex,flow-control>)"
// "media: autoselect (10GbaseT <full-duplex>)", "(100baseTX <half-duplex>)"
var macMediaRe = regexp.MustCompile(`media:.*\((\d+(?:\.\d+)?)(G?)[bB]ase[^\s<)]*\s*(?:<([^>]*)>)?`)

// parseMacMedia returns link speed in Mb/s and "full"/"half"/"" from the
// output of "ifconfig <device>".
func parseMacMedia(ifconfigOut string) (int, string) {
	m := macMediaRe.FindStringSubmatch(ifconfigOut)
	if m == nil {
		return 0, ""
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	if m[2] == "G" {
		v *= 1000
	}
	duplex := ""
	switch {
	case strings.Contains(m[3], "full-duplex"):
		duplex = "full"
	case strings.Contains(m[3], "half-duplex"):
		duplex = "half"
	}
	return int(v), duplex
}

// parseMacHardwarePorts reads "networksetup -listallhardwareports" into
// device -> hardware port name ("en0" -> "Wi-Fi").
func parseMacHardwarePorts(out string) map[string]string {
	ports := map[string]string{}
	port := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Hardware Port:"):
			port = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		case strings.HasPrefix(line, "Device:") && port != "":
			ports[strings.TrimSpace(strings.TrimPrefix(line, "Device:"))] = port
			port = ""
		}
	}
	return ports
}

func isMacWiFiPort(port string) bool {
	p := strings.ToLower(port)
	return strings.Contains(p, "wi-fi") || strings.Contains(p, "airport") || strings.Contains(p, "wlan")
}
