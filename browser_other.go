//go:build !windows

package main

import (
	"os"
	"os/exec"
	"runtime"
)

func openBrowser(url string) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	} else if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return // headless Linux box: the URL is printed in the console
	}
	_ = exec.Command(name, url).Start()
}
