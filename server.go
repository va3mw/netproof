package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Request is the first JSON line on every TCP connection.
type Request struct {
	Magic      string  `json:"magic"`
	Op         string  `json:"op"` // hello, start, download, upload, ping, report
	DurationMs int     `json:"durationMs,omitempty"`
	FromHost   string  `json:"fromHost,omitempty"`
	Result     *Result `json:"result,omitempty"`
}

// HelloReply describes the far end.
type HelloReply struct {
	Magic    string `json:"magic"`
	ID       string `json:"id"`
	App      string `json:"app"`
	Version  string `json:"version"`
	Host     string `json:"host"`
	OS       string `json:"os"`
	Busy     bool   `json:"busy"`
	Iface    string `json:"iface"`
	Kind     string `json:"kind"`
	LinkMbps int    `json:"linkMbps"`
	Duplex   string `json:"duplex,omitempty"`
}

type UploadReply struct {
	Bytes         int64   `json:"bytes"`
	MeasuredBytes int64   `json:"measuredBytes"`
	MeasuredSec   float64 `json:"measuredSec"`
}

type Peer struct {
	ID       string    `json:"id"`
	Addr     string    `json:"addr"` // host or host:port to test against
	IP       string    `json:"ip"`
	Host     string    `json:"host"`
	OS       string    `json:"os"`
	Via      string    `json:"via"` // "LAN" or "Tailscale"
	LastSeen time.Time `json:"-"`
}

// Incoming describes a test someone else is running against us.
type Incoming struct {
	FromHost string    `json:"fromHost"`
	FromIP   string    `json:"fromIP"`
	Started  time.Time `json:"started"`
	Result   *Result   `json:"result,omitempty"`
}

type App struct {
	mu       sync.Mutex
	id       string
	host     string
	listenIP string
	port     int
	discPort int
	bindIP   string
	profile  string
	noBind   bool // true while testing over loopback (practice)

	peers    map[string]*Peer
	test     TestState
	cancel   context.CancelFunc
	incoming *Incoming
	lastRes  *Result

	ifCache   []IfaceInfo
	ifCacheAt time.Time
	disc      *net.UDPConn
}

func newApp(listen string, port, discPort int) *App {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	h, _ := os.Hostname()
	if h == "" {
		h = "unknown"
	}
	return &App{
		id:       hex.EncodeToString(b),
		host:     h,
		listenIP: listen,
		port:     port,
		discPort: discPort,
		profile:  "auto",
		peers:    map[string]*Peer{},
		test:     TestState{PhaseLabel: "Ready"},
	}
}

func (a *App) startServers() error {
	addr := net.JoinHostPort(a.listenIP, fmt.Sprint(a.port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d is in use (%v)", a.port, err)
	}
	ua, _ := net.ResolveUDPAddr("udp", addr)
	uc, err := net.ListenUDP("udp", ua)
	if err != nil {
		ln.Close()
		return fmt.Errorf("UDP port %d is in use (%v)", a.port, err)
	}
	go a.serveTCP(ln)
	go a.serveUDPEcho(uc)
	return nil
}

func (a *App) serveTCP(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		go a.handle(c)
	}
}

func (a *App) serveUDPEcho(uc *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, from, err := uc.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		if n >= 4 && bytes.Equal(buf[:4], []byte("NPRF")) {
			_, _ = uc.WriteToUDP(buf[:n], from)
		}
	}
}

func writeJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func readJSONLine(br *bufio.Reader, v any, max int) error {
	var line []byte
	for {
		part, isPrefix, err := br.ReadLine()
		if err != nil {
			return err
		}
		line = append(line, part...)
		if len(line) > max {
			return errors.New("line too long")
		}
		if !isPrefix {
			break
		}
	}
	return json.Unmarshal(line, v)
}

func (a *App) helloReply(local net.Addr) HelloReply {
	r := HelloReply{Magic: magic, ID: a.id, App: appName, Version: appVersion, Host: a.host, OS: runtime.GOOS}
	a.mu.Lock()
	r.Busy = a.test.Running
	a.mu.Unlock()
	if ta, ok := local.(*net.TCPAddr); ok {
		if ifc, ok := a.ifaceForIP(ta.IP.String()); ok {
			r.Iface, r.Kind, r.LinkMbps, r.Duplex = ifc.Name, ifc.Kind, ifc.LinkMbps, ifc.Duplex
		}
	}
	return r
}

func remoteIP(c net.Conn) string {
	if ta, ok := c.RemoteAddr().(*net.TCPAddr); ok {
		return ta.IP.String()
	}
	return c.RemoteAddr().String()
}

func (a *App) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	br := bufio.NewReaderSize(c, 64*1024)
	var req Request
	if err := readJSONLine(br, &req, 1<<20); err != nil || req.Magic != magic {
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	dur := time.Duration(req.DurationMs) * time.Millisecond
	if dur < time.Second {
		dur = time.Second
	}
	if dur > 30*time.Second {
		dur = 30 * time.Second
	}

	switch req.Op {
	case "hello":
		_ = writeJSONLine(c, a.helloReply(c.LocalAddr()))

	case "start":
		a.mu.Lock()
		a.incoming = &Incoming{FromHost: req.FromHost, FromIP: remoteIP(c), Started: time.Now()}
		a.mu.Unlock()
		fmt.Printf("[%s] Test started by %s (%s)\n", time.Now().Format("15:04:05"), req.FromHost, remoteIP(c))
		_ = writeJSONLine(c, a.helloReply(c.LocalAddr()))

	case "report":
		if req.Result == nil {
			return
		}
		a.mu.Lock()
		if a.incoming == nil {
			a.incoming = &Incoming{FromHost: req.FromHost, FromIP: remoteIP(c), Started: time.Now()}
		}
		a.incoming.Result = req.Result
		a.mu.Unlock()
		fmt.Printf("[%s] Results from %s:\n%s\n", time.Now().Format("15:04:05"), req.FromHost, req.Result.Text())
		_, _ = c.Write([]byte("{}\n"))

	case "download": // we send, they receive
		tuneTCP(c)
		buf := make([]byte, 128*1024)
		_, _ = rand.Read(buf)
		end := time.Now().Add(dur)
		_ = c.SetWriteDeadline(end.Add(5 * time.Second))
		for time.Now().Before(end) {
			if _, err := c.Write(buf); err != nil {
				return
			}
		}

	case "upload": // they send, we receive and report what actually arrived
		tuneTCP(c)
		_ = c.SetReadDeadline(time.Now().Add(dur + 20*time.Second))
		buf := make([]byte, 128*1024)
		var total, atWarm int64
		var first, warm time.Time
		for {
			n, err := br.Read(buf)
			if n > 0 {
				now := time.Now()
				if first.IsZero() {
					first = now
				}
				total += int64(n)
				if warm.IsZero() && now.Sub(first) >= warmup {
					warm, atWarm = now, total
				}
			}
			if err != nil {
				break
			}
		}
		end := time.Now()
		rep := UploadReply{Bytes: total}
		if !warm.IsZero() && end.Sub(warm) > 200*time.Millisecond {
			rep.MeasuredBytes, rep.MeasuredSec = total-atWarm, end.Sub(warm).Seconds()
		} else if !first.IsZero() {
			rep.MeasuredBytes, rep.MeasuredSec = total, end.Sub(first).Seconds()
		}
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = writeJSONLine(c, rep)

	case "ping": // TCP echo, used when UDP is blocked
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		b := make([]byte, 16)
		for {
			_ = c.SetReadDeadline(time.Now().Add(15 * time.Second))
			if _, err := io.ReadFull(br, b); err != nil {
				return
			}
			if _, err := c.Write(b); err != nil {
				return
			}
		}
	}
}

func tuneTCP(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(4 << 20)
		_ = tc.SetWriteBuffer(4 << 20)
	}
}

// ---------------------------------------------------------------- discovery

type announce struct {
	Magic string `json:"magic"`
	ID    string `json:"id"`
	Host  string `json:"host"`
	OS    string `json:"os"`
	Port  int    `json:"port"`
}

// discoveryLoop broadcasts "I'm here" on every LAN and listens for others.
func (a *App) discoveryLoop() {
	uc, err := net.ListenUDP("udp4", &net.UDPAddr{Port: a.discPort})
	if err != nil {
		fmt.Println("Note: automatic LAN discovery is off (", err, "). Type the other computer's address instead.")
		return
	}
	a.disc = uc
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := uc.ReadFromUDP(buf)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			var an announce
			if json.Unmarshal(buf[:n], &an) != nil || an.Magic != magic || an.ID == a.id {
				continue
			}
			ip := from.IP.String()
			addr := ip
			if an.Port != 0 && an.Port != defDataPort {
				addr = net.JoinHostPort(ip, fmt.Sprint(an.Port))
			}
			a.addPeer(&Peer{ID: an.ID, Addr: addr, IP: ip, Host: an.Host, OS: an.OS, Via: "LAN"})
		}
	}()
	msg, _ := json.Marshal(announce{Magic: magic, ID: a.id, Host: a.host, OS: runtime.GOOS, Port: a.port})
	for {
		targets := map[string]bool{"255.255.255.255": true}
		for _, b := range broadcastAddrs() {
			targets[b] = true
		}
		for t := range targets {
			_, _ = uc.WriteToUDP(msg, &net.UDPAddr{IP: net.ParseIP(t), Port: a.discPort})
		}
		time.Sleep(2 * time.Second)
	}
}

func (a *App) addPeer(p *Peer) {
	p.LastSeen = time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	key := p.IP
	if old, ok := a.peers[key]; ok && old.Via == "Tailscale" && p.Via == "LAN" {
		p.Via = old.Via // keep the more specific label
	}
	a.peers[key] = p
}

func (a *App) peerList() []Peer {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []Peer
	for k, p := range a.peers {
		max := 8 * time.Second
		if p.Via == "Tailscale" || p.Via == "This computer" {
			max = 60 * time.Second
		}
		if time.Since(p.LastSeen) > max {
			delete(a.peers, k)
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Via != out[j].Via {
			return out[i].Via == "LAN"
		}
		return strings.ToLower(out[i].Host) < strings.ToLower(out[j].Host)
	})
	return out
}

// ---------------------------------------------------------------- tailscale

func findTailscale() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		cands = []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Tailscale", "tailscale.exe"),
			`C:\Program Files\Tailscale\tailscale.exe`,
		}
	case "darwin":
		cands = []string{"/Applications/Tailscale.app/Contents/MacOS/Tailscale"}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// tailscaleLoop finds Tailscale peers (broadcast does not cross VPNs) and
// checks which of them are running NetProof.
func (a *App) tailscaleLoop() {
	ts := findTailscale()
	if ts == "" {
		return
	}
	for {
		a.scanTailscale(ts)
		time.Sleep(20 * time.Second)
	}
}

func (a *App) scanTailscale(ts string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ts, "status", "--json")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	var st struct {
		Peer map[string]struct {
			HostName     string
			TailscaleIPs []string
			Online       bool
		}
	}
	if json.Unmarshal(out, &st) != nil {
		return
	}
	var wg sync.WaitGroup
	for _, p := range st.Peer {
		if !p.Online {
			continue
		}
		var ip string
		for _, x := range p.TailscaleIPs {
			if v := net.ParseIP(x); v != nil && v.To4() != nil {
				ip = x
			}
		}
		if ip == "" {
			continue
		}
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			h, err := a.hello(ip, a.port, 1500*time.Millisecond)
			if err != nil {
				return
			}
			a.addPeer(&Peer{ID: "ts-" + ip, Addr: ip, IP: ip, Host: h.Host, OS: h.OS, Via: "Tailscale"})
		}(ip)
	}
	wg.Wait()
}

// practicePeerLoop lists a second copy running on this same computer
// (started with --second) so new users can practise with two windows.
func (a *App) practicePeerLoop() {
	other := defDataPort
	if a.port == defDataPort {
		other = secondDataPort
	}
	for {
		if h, err := a.hello("127.0.0.1", other, time.Second); err == nil && h.ID != a.id {
			a.addPeer(&Peer{ID: h.ID, Addr: fmt.Sprintf("127.0.0.1:%d", other), IP: fmt.Sprintf("127.0.0.1:%d", other),
				Host: h.Host + " (" + copyName(other) + ")", OS: h.OS, Via: "This computer"})
		}
		time.Sleep(3 * time.Second)
	}
}

func copyName(port int) string {
	if port == secondDataPort {
		return "second copy"
	}
	return "first copy"
}

// isNetProofUI checks whether another copy already serves the UI there.
func isNetProofUI(url string) bool {
	c := &httpClient
	resp, err := c.Get(url + "api/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 100))
	return strings.Contains(string(b), appName)
}
