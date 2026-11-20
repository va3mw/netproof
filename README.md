# NetProof

Network speed, delay and jitter tester that anyone can run. One file, nothing to install.

## For users

1. Run NetProof on **both** computers (`netproof.exe` on Windows, `./netproof-linux-amd64` on Debian).
   A browser window opens by itself.
2. On **one** of them, click **Test** next to the other computer's name.
   If it is not listed (VPN, Tailscale, another subnet), type its address and click **Start test**.
3. Wait about 30 seconds. Both screens show the verdict.

You do not need to choose "client" or "server". Whichever computer presses Test drives the test;
the other one answers automatically.

**Windows:** the first time, Windows Firewall asks whether to allow NetProof. Tick **both** Private
and Public and click **Allow**. If you skip this, the other computer cannot reach this one.

**Debian:** `chmod +x netproof-linux-amd64` once. On a machine with no desktop, it just prints
its address; run the test from the other computer, or from the command line:
`./netproof-linux-amd64 192.168.1.50`. If `ufw` is on: `sudo ufw allow 47800:47801/tcp` and
`sudo ufw allow 47800:47801/udp`.

## Practising on one computer

Two ways, so new users can get used to it before involving a second machine:

* **Practice run button.** Start NetProof and click **Practice run**. It tests the computer
  against itself. Results are shown but not graded, because no network is involved.
* **Two copies side by side.** Start NetProof normally, then start a second copy with
  `netproof --second` (on Windows, double-click `NetProof second copy.bat` placed next to
  `netproof.exe`). The two copies list each other as "first copy" and "second copy" under
  "This computer"; press **Test** in either window and watch both screens, just like a real test.
  The second copy uses ports 47810 and 47890.

## What it measures (about 30 s total)

| Phase | Time | How |
|---|---|---|
| Delay, jitter, loss | 6 s | 300 UDP probes (falls back to TCP if UDP is blocked) |
| Download | 10 s | 4 parallel TCP streams, far end to this end, first second ignored |
| Upload | 10 s | same, other direction, measured at the receiving end |
| Busy delay | during both | 10 probes/s while the link is full (bufferbloat) |

## Reference values (healthy network)

| | 100 Mb/s LAN | 1 Gb/s LAN | Wi-Fi | VPN / Tailscale |
|---|---|---|---|---|
| Speed each way | 90 to 95 Mb/s | 900 to 945 Mb/s | 300+ Mb/s | no fixed target |
| Problem below | 85 | 850 | 100 | |
| Delay | < 1 ms | < 1 ms | < 4 ms | < 30 ms |
| Jitter | < 0.5 ms | < 0.5 ms | < 2 ms | < 3 ms |
| Loss | 0 % | 0 % | < 0.1 % | < 0.1 % |
| Extra delay when busy | < 20 ms | < 10 ms | < 30 ms | < 30 ms |

Before testing, users can say what kind of connection it is ("What kind of connection is
this?": gigabit LAN, 100 Mb LAN, Wi-Fi, or VPN / remote site / internet), or leave it on
Automatic. Automatic treats the link as a VPN or remote site when the other computer is on a
different subnet and delay is above 3 ms (site-to-site VPNs look like plain Ethernet otherwise),
or when delay is above 5 ms on any wired link, and shows the reason with the results.

The expected profile is picked automatically from the adapters' link speed and type (wired,
Wi-Fi, VPN, Tailscale 100.x addresses) and can be changed on the results screen. It also flags
adapters stuck at 100 Mb/s or 10 Mb/s, half duplex, and lopsided up/down speeds.

## Ports

TCP + UDP 47800 (tests), UDP 47801 (LAN discovery broadcast), 127.0.0.1:47880 (local browser UI only).

## Building

Requires Go 1.22+. `./build.sh` produces everything in `dist/`. Mac: `GOOS=darwin GOARCH=arm64 go build`
already works; link speed detection on Mac is still a stub (`linkinfo_other.go`).
