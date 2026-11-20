# NetProof

Network speed, delay and jitter tester that anyone can run. One file, nothing to install.

## For users

1. Run NetProof on **both** computers (`netproof.exe` on Windows, `./netproof-linux-amd64` on Debian).
   A browser window opens by itself.
2. On **one** of them, click **Test** next to the other computer's name.
   If it is not listed (VPN, Tailscale, another subnet), type its address or name and click **Start test**.
   Tailscale MagicDNS names work: `boathouse2` or `boathouse2.tail1234.ts.net`.
3. Wait about 45 seconds. Both screens show the verdict and a "How to fix it" checklist.

You do not need to choose "client" or "server". Whichever computer presses Test drives the test;
the other one answers automatically.

**Windows:** the first time, Windows Firewall asks whether to allow NetProof. Tick **both** Private
and Public and click **Allow**. If you skip this, the other computer cannot reach this one.

**Debian:** `chmod +x netproof-linux-amd64` once. On a machine with no desktop, it just prints
its address; run the test from the other computer, or from the command line:
`./netproof-linux-amd64 192.168.1.50`. If `ufw` is on: `sudo ufw allow 47800:47801/tcp` and
`sudo ufw allow 47800:47801/udp`.

**"Contains a virus or potentially unwanted software":** this is a false positive from
Microsoft Defender, common for new, unsigned programs. NetProof avoids the usual triggers
(no PowerShell, no rundll32, not stripped, full version info), but only code signing
removes it for good. Until then: Windows Security > Virus & threat protection > Protection
history > select the NetProof entry > Actions > Allow / Restore. Report the false positive at
https://www.microsoft.com/wdsi/filesubmission ("Software developer", "Incorrectly detected").

## Practising on one computer

Two ways, so new users can get used to it before involving a second machine:

* **Practice run button.** Start NetProof and click **Practice run**. It tests the computer
  against itself. Results are shown but not graded, because no network is involved.
* **Two copies side by side.** Start NetProof normally, then start a second copy with
  `netproof --second` (on Windows, double-click `NetProof second copy.bat` placed next to
  `netproof.exe`). The two copies list each other as "first copy" and "second copy" under
  "This computer"; press **Test** in either window and watch both screens, just like a real test.
  The second copy uses ports 47810 and 47890.

## What it measures (about 45 s total)

| Phase | Time | How |
|---|---|---|
| Delay, jitter, loss | 6 s | 300 small UDP probes (falls back to TCP if UDP is blocked) |
| Packet size (MTU) | ~1 s | largest UDP packet that crosses unsplit (Don't Fragment), and whether split packets get through |
| Download | 10 s | 4 parallel TCP streams, far end to this end, first second ignored |
| Upload | 10 s | same, other direction, measured at the receiving end |
| Busy delay | during both | 10 probes/s while the link is full (bufferbloat) |
| UDP stream in | 8 s | steady 1200 byte UDP packets at a fixed rate, like radio audio, panadapter and IQ streams: loss, one-way jitter, out of order |
| UDP stream out | 8 s | same, other direction |

UDP stream rate: 100 Mb/s on a gigabit LAN, 30 Mb/s on 100 Mb or Wi-Fi, 10 Mb/s over a VPN,
never more than 70% of the TCP speed just measured (so a slow link is tested for loss, not overrun).
A computer only streams to an address that has just connected to it over TCP.

## Diagnosis

Every problem found comes with a "How to fix it" checklist in plain language: link stuck at
100 Mb/s, duplex, packet loss on a wired LAN (cables, RF pickup, switches), slow LAN, Wi-Fi,
bufferbloat (SQM / cake / fq_codel), jitter, VPN path (relays, Tailscale DERP), MTU and
fragmentation (MSS clamping, tunnel MTU), and UDP blocked by firewalls. The checklists are also
in the saved and copied text report.

## How results are judged

NetProof only calls something out when a person would actually hear or feel it. Findings are
grouped as **You will notice** (the only thing that makes the verdict "problems"), **You may
notice**, and **Good to know**, and each one says what you will experience first (clicks and
dropouts, laggy remote control, slow file copies) and the numbers second.

| | Healthy 100 Mb/s LAN | Healthy 1 Gb/s LAN | Healthy VPN | You may notice | You will notice |
|---|---|---|---|---|---|
| Speed each way | 90 to 95 Mb/s | 900 to 945 Mb/s | depends on both internet links | below 80 / 700 / 10 Mb/s | below 40 / 300 / 3 Mb/s |
| Delay (round trip) | < 1 ms | < 1 ms | 10 to 40 ms | > 50 ms | > 150 ms |
| Jitter | < 1 ms | < 1 ms | < 3 ms | > 5 ms | > 15 ms |
| Stream loss | 0 % | 0 % | 0 % | > 0.05 % | > 0.5 % |
| Extra delay when busy | < 20 ms | < 10 ms | < 30 ms | > 50 ms (LAN: > 100 ms) | > 150 ms (never on a LAN) |
| Largest packet (MTU) | 1500 | 1500 | 1280 to 1420 | < 1280 | |

Only speed depends on the kind of link. Delay, jitter, loss and busy delay use the same
"would you notice" limits everywhere: audio stutters the same whether jitter came from Wi-Fi or
a VPN. If one computer is gigabit and the other is stuck at 100 Mb/s, Automatic judges the link
as gigabit and points at the slow end.

## Tested before

NetProof remembers the last 10 computers it tested, and those that tested it, under
"Tested before", with the last verdict and a green dot when NetProof is running there now.
Saved in `%AppData%\NetProof\recent.json` (Windows) or `~/.config/NetProof/recent.json` (Linux).
Click × to forget one.

## Names and Tailscale MagicDNS

A typed name is resolved once at the start of the test (IPv4 preferred, so every part of the
test takes the same path). If the computer's DNS does not know the name, NetProof looks it up in
Tailscale itself (short MagicDNS name, full `.ts.net` name or Tailscale host name), so MagicDNS
names work even when "Use Tailscale DNS" is off or on Linux without Tailscale DNS. Tailscale
peers running NetProof are listed automatically with their MagicDNS names.

## Ports

TCP + UDP 47800 (tests), UDP 47801 (LAN discovery broadcast), 127.0.0.1:47880 (local browser UI only).

## Building

Requires Go 1.22+. Windows icon and version info: `go-winres simply --arch amd64,arm64 --out rsrc --manifest cli --product-name NetProof --product-version X.Y.Z.0 --file-version X.Y.Z.0 --icon winres/icon.png` (from github.com/tc-hib/go-winres). `./build.sh` produces everything in `dist/`. Mac: `GOOS=darwin GOARCH=arm64 go build`
already works; link speed detection on Mac is still a stub (`linkinfo_other.go`).
