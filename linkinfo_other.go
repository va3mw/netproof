//go:build !linux && !windows && !darwin

package main

// macOS and others: link speed detection to be added (networksetup / ifconfig).
func osLinkInfo() map[string]osIfInfo { return map[string]osIfInfo{} }
