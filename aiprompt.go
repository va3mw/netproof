package main

// The speed graph kept with a result, and the "Ask an AI" prompt: the whole
// report wrapped in instructions, for people who want to talk it through
// with ChatGPT, Claude or similar.

import (
	"fmt"
	"math"
	"strings"
)

var speedPhases = []string{"download", "upload", "udpdown", "udpup"}

// speedSamples keeps the throughput samples (what the graph draws), evenly
// thinned to at most max points so a result stays small on the wire.
//
// The first second of each TCP phase is left out, as the measurement does:
// the upload is counted as it is written to the socket, so its first moments
// show the send buffers filling (hundreds of Mb/s that never crossed the
// network), not the link.
func speedSamples(all []Sample, max int) []Sample {
	var thr []Sample
	phaseStart := map[string]float64{}
	for _, s := range all {
		if _, ok := phaseStart[s.Phase]; !ok {
			phaseStart[s.Phase] = s.T
		}
		if (s.Phase == "download" || s.Phase == "upload") && s.T-phaseStart[s.Phase] < warmup.Seconds() {
			continue
		}
		for _, p := range speedPhases {
			if s.Phase == p {
				thr = append(thr, Sample{T: math.Round(s.T*100) / 100, Mbps: math.Round(s.Mbps*10) / 10, Phase: s.Phase})
				break
			}
		}
	}
	if len(thr) <= max {
		return thr
	}
	out := make([]Sample, 0, max)
	step := float64(len(thr)) / float64(max)
	for i := 0; i < max; i++ {
		out = append(out, thr[int(float64(i)*step)])
	}
	return out
}

// speedSummary describes each speed phase of the graph in one line: lowest,
// average and highest, and how steady it was. Text, so an AI can read it.
func (r *Result) speedSummary() string {
	names := map[string]string{
		"download": "TCP " + r.RemoteHost + " -> " + r.LocalHost,
		"upload":   "TCP " + r.LocalHost + " -> " + r.RemoteHost,
		"udpdown":  "UDP stream " + r.RemoteHost + " -> " + r.LocalHost,
		"udpup":    "UDP stream " + r.LocalHost + " -> " + r.RemoteHost,
	}
	var b strings.Builder
	for _, p := range speedPhases {
		var vals []float64
		for _, s := range r.Samples {
			if s.Phase == p && s.Mbps > 0 {
				vals = append(vals, s.Mbps)
			}
		}
		if len(vals) < 3 {
			continue
		}
		lo, hi, sum := vals[0], vals[0], 0.0
		for _, v := range vals {
			lo, hi, sum = math.Min(lo, v), math.Max(hi, v), sum+v
		}
		avg := sum / float64(len(vals))
		sd := 0.0
		for _, v := range vals {
			sd += (v - avg) * (v - avg)
		}
		sd = math.Sqrt(sd / float64(len(vals)))
		fmt.Fprintf(&b, "  %s: lowest %s, average %s, highest %s, varied by %.0f%% (%d samples)\n",
			names[p], mbps(lo), mbps(avg), mbps(hi), 100*sd/math.Max(avg, 0.001), len(vals))
	}
	return b.String()
}

// AIPrompt is a self-contained prompt to paste into an AI assistant.
func (r *Result) AIPrompt() string {
	var b strings.Builder
	b.WriteString(`I tested the network between two of my computers with NetProof, a network tester for ham radio operators. It measures speed both ways, delay, jitter, packet loss, packet size (MTU), delay while the link is busy (bufferbloat) and a steady UDP stream both ways, like remote radio audio, panadapter and SDR data. I want help understanding the results and fixing anything that would affect remote radio operation.

Please:
1. Explain in plain language what these results mean for remote radio use (audio, panadapter, CAT and remote control), and what I would actually notice.
2. Name the most likely causes of any problem, most likely first, and say which numbers point to each.
3. Before giving detailed fixes, ask me the questions you need answered (for example: router and switch makes and models, Wi-Fi or cable at each end, VPN or Tailscale in use, ISP and line type, what else uses the network, which radio software I run).
4. Then give step-by-step fixes, simplest and cheapest first, and tell me what to re-test with NetProof after each change to confirm it worked.

Base everything on the numbers below; do not assume measurements that are not there. NetProof's own findings and fix list are included at the end of the report, and you may agree or disagree with them.

`)
	if r.Practice {
		b.WriteString("Note: this was a practice run, with the computer testing itself, so the numbers only show this computer's own network stack, not a real link.\n\n")
	}
	b.WriteString("----- NetProof report -----\n")
	b.WriteString(r.Text())
	if s := r.speedSummary(); s != "" {
		b.WriteString("\nSpeed over time during each phase (from the graph):\n")
		b.WriteString(s)
	}
	b.WriteString("----- end of report -----\n")
	return b.String()
}
