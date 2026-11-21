//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

const isWindows = false

// runningPrograms lists the names of running processes from /proc (Linux).
// On macOS there is no /proc, so it finds nothing.
func runningPrograms() []string {
	dirs, _ := filepath.Glob("/proc/[0-9]*/comm")
	var out []string
	for _, d := range dirs {
		if b, err := os.ReadFile(d); err == nil {
			out = append(out, strings.TrimSpace(string(b)))
		}
	}
	return out
}
