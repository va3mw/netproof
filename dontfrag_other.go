//go:build !linux && !windows

package main

import (
	"net"
	"syscall"
)

const ipDontFrag = 28 // IP_DONTFRAG on macOS and BSD

func setDontFragment(uc *net.UDPConn) error {
	rc, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var serr error
	err = rc.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, ipDontFrag, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
