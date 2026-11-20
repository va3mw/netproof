//go:build windows

package main

import (
	"net"
	"syscall"
)

const ipDontFragment = 14 // IP_DONTFRAGMENT, IPPROTO_IP level

func setDontFragment(uc *net.UDPConn) error {
	rc, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var serr error
	err = rc.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, ipDontFragment, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
