//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// swapAndRestart: a running .exe cannot be overwritten on Windows, but it can
// be renamed. Move it aside, put the new one in its place and start it in
// this same console window, so the person sees it carry on where it was
// (starting it any other way opens a new window and drops the old one back
// to its command prompt). This copy then lets go of its ports, stays out of
// the way and exits when the new one does, with the same exit code.
func (a *App) swapAndRestart(exe, newPath string) error {
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(newPath, exe); err != nil {
		_ = os.Rename(old, exe) // put it back
		return err
	}
	cmd := exec.Command(exe, restartArgs()...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.Rename(exe, newPath)
		_ = os.Rename(old, exe)
		return err
	}
	// Ctrl+C and closing the window reach the new copy too; let it handle
	// them and only follow it out.
	signal.Ignore(os.Interrupt, syscall.SIGTERM)
	a.releasePorts()
	code := 0
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	os.Exit(code)
	return nil
}
