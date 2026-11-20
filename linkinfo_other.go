//go:build !linux && !windows

package main

import "os/exec"

// macOS and others: link speed detection to be added (networksetup / ifconfig).
func osLinkInfo() map[string]osIfInfo { return map[string]osIfInfo{} }

func hideWindow(cmd *exec.Cmd) {}

// openURL is only needed on Windows; other systems use a launcher command.
func openURL(string) {}
