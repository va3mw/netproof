// NetProof: a dead simple LAN / VPN speed, latency and jitter tester.
//
// Run the same program on two computers. Both sides always listen; whichever
// side presses "Test" becomes the client and the other side automatically acts
// as the receiver. Results show on both screens.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	appName     = "NetProof"
	appVersion  = "1.3.0"
	defDataPort = 47800 // TCP + UDP: test traffic and control
	defDiscPort = 47801 // UDP: LAN discovery broadcasts
	defUIPort   = 47880 // HTTP on 127.0.0.1: the user interface

	// A second copy on the same computer (--second) uses these instead.
	secondDataPort = 47810
	secondUIPort   = 47890
	magic          = "NPRF1"
)

func main() {
	noBrowser := flag.Bool("no-browser", false, "do not open the web browser automatically")
	profile := flag.String("profile", "auto", "expected connection type: auto, 1g, 100m, wifi, vpn")
	bind := flag.String("bind", "", "local IP address (network interface) to test from")
	listen := flag.String("listen", "", "IP address to listen on (default: all)")
	port := flag.Int("port", defDataPort, "test port (TCP and UDP)")
	discPort := flag.Int("discovery-port", defDiscPort, "LAN discovery port (UDP)")
	uiPort := flag.Int("ui-port", defUIPort, "web interface port on 127.0.0.1")
	second := flag.Bool("second", false, "run a second copy on this computer, for practice")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s %s: network speed, latency and jitter tester\n\n", appName, appVersion)
		fmt.Fprintf(os.Stderr, "Usage:\n  netproof                 start and open the browser window (normal use)\n")
		fmt.Fprintf(os.Stderr, "  netproof <ip-or-name>    run a test from the command line and print results\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *second {
		*port, *uiPort = secondDataPort, secondUIPort
	}

	if _, ok := profiles[*profile]; !ok && *profile != "auto" {
		fmt.Fprintf(os.Stderr, "unknown profile %q (use auto, 1g, 100m, wifi or vpn)\n", *profile)
		os.Exit(2)
	}

	a := newApp(*listen, *port, *discPort)
	a.profile = *profile
	if *bind != "" {
		a.bindIP = *bind
	}

	// Command line mode: netproof <target>
	if flag.NArg() > 0 {
		os.Exit(a.runCLI(flag.Arg(0)))
	}

	if err := a.startServers(); err != nil {
		// Most likely another copy is already running: just show its window.
		uiURL := "http://127.0.0.1:" + strconv.Itoa(*uiPort) + "/"
		if isNetProofUI(uiURL) {
			fmt.Println("NetProof is already running. Opening its window:", uiURL)
			openBrowser(uiURL)
			return
		}
		fmt.Fprintln(os.Stderr, "Could not start:", err)
		pauseOnWindows()
		os.Exit(1)
	}
	go a.discoveryLoop()
	go a.tailscaleLoop()
	go a.practicePeerLoop()

	uiURL, err := a.startUI(*uiPort)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not start the user interface:", err)
		pauseOnWindows()
		os.Exit(1)
	}

	fmt.Println("==========================================================")
	fmt.Printf(" %s %s is running on %s\n", appName, appVersion, a.host)
	fmt.Println("")
	fmt.Println(" Open this in your web browser if it did not open itself:")
	fmt.Println("   " + uiURL)
	fmt.Println("")
	if *second {
		fmt.Println(" This is the SECOND (practice) copy. The first copy will list it")
		fmt.Println(" as \"" + a.host + " (second copy)\". Press Test there.")
	} else {
		fmt.Println(" Run NetProof on the other computer too, then press Test.")
		fmt.Println(" Just practising? Press \"Practice run\" in the browser window.")
	}
	fmt.Println(" Keep this window open. Close it (or press Ctrl+C) to stop.")
	fmt.Println("==========================================================")
	if ips := a.localIPv4s(); len(ips) > 0 {
		fmt.Println(" This computer's addresses: " + strings.Join(ips, ", "))
	}
	if !*noBrowser {
		openBrowser(uiURL)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("\nStopped.")
}

func (a *App) runCLI(target string) int {
	// Listen too, so UDP echo from the far side has somewhere to go if needed.
	fmt.Printf("%s %s: testing %s (about 45 seconds)\n", appName, appVersion, target)
	last := ""
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				st := a.snapshotTest()
				line := fmt.Sprintf("  %-28s %5.0f%%", st.PhaseLabel, st.Progress*100)
				if st.CurrentMbps > 0 {
					line += fmt.Sprintf("   %8.1f Mb/s", st.CurrentMbps)
				}
				if line != last {
					fmt.Println(line)
					last = line
				}
			}
		}
	}()
	res, err := a.runTest(context.Background(), target)
	close(done)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nTest failed:", err)
		return 1
	}
	fmt.Println()
	fmt.Println(res.Text())
	return 0
}

func pauseOnWindows() {
	if runtime.GOOS == "windows" {
		fmt.Println("Press Enter to close.")
		fmt.Scanln()
	}
}

func (a *App) localIPv4s() []string {
	var out []string
	for _, ifc := range a.interfaces() {
		out = append(out, ifc.IP+" ("+ifc.Name+")")
	}
	return out
}

// splitTarget turns "host", "host:port" or "[v6]:port" into host and port.
func (a *App) splitTarget(t string) (string, int) {
	t = strings.TrimSpace(t)
	if h, p, err := net.SplitHostPort(t); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			return h, n
		}
	}
	return strings.Trim(t, "[]"), a.port
}
