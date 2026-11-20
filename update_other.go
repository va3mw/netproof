//go:build !windows

package main

import (
	"os"
	"syscall"
)

// swapAndRestart: on Linux and macOS the program file can be replaced while
// it runs. Rename the new one over it, then exec it in place: same process,
// and the old sockets close on exec, freeing the ports.
func (a *App) swapAndRestart(exe, newPath string) error {
	if err := os.Rename(newPath, exe); err != nil {
		return err
	}
	return syscall.Exec(exe, append([]string{exe}, restartArgs()...), os.Environ())
}
