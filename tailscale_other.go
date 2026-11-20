//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
)

func findTailscale() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	for _, c := range []string{"/Applications/Tailscale.app/Contents/MacOS/Tailscale"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func tailscaleInstalled() bool { return findTailscale() != "" }

func tailscaleStatusJSON(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, findTailscale(), "status", "--json").Output()
}
