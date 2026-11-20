//go:build windows

package main

// On Windows, Tailscale peers are read from the Tailscale service's local
// API over its named pipe, instead of running tailscale.exe. Keeping the
// Windows build free of any "start another program" code avoids antivirus
// machine-learning false positives (Trojan:Win32/Bearfoos.A!ml).

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const tailscalePipe = `\\.\pipe\ProtectedPrefix\Administrators\Tailscale\tailscaled`

func tailscaleInstalled() bool {
	_, err := os.Stat(filepath.Join(os.Getenv("ProgramFiles"), "Tailscale"))
	return err == nil
}

func tailscaleStatusJSON(ctx context.Context) ([]byte, error) {
	f, err := os.OpenFile(tailscalePipe, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		req, _ := http.NewRequest("GET", "http://local-tailscaled.sock/localapi/v0/status", nil)
		req.Header.Set("Connection", "close")
		if err := req.Write(f); err != nil {
			ch <- result{nil, err}
			return
		}
		resp, err := http.ReadResponse(bufio.NewReader(f), req)
		if err != nil {
			ch <- result{nil, err}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			ch <- result{nil, errors.New(resp.Status)}
			return
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		ch <- result{b, err}
	}()
	select {
	case r := <-ch:
		f.Close()
		return r.b, r.err
	case <-ctx.Done():
		f.Close() // unblocks the reader
		return nil, ctx.Err()
	case <-time.After(8 * time.Second):
		f.Close()
		return nil, errors.New("tailscale did not answer")
	}
}
