//go:build linux

package main

import (
	"net"
	"syscall"
)

// setDontFragment sets DF and ignores any cached path MTU, so probes measure
// the real path (IP_PMTUDISC_PROBE).
func setDontFragment(uc *net.UDPConn) error {
	rc, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var serr error
	err = rc.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_MTU_DISCOVER, 3 /* IP_PMTUDISC_PROBE */)
	})
	if err != nil {
		return err
	}
	return serr
}
