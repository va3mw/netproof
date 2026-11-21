package main

import "testing"

// The upload's first second (send buffers filling) must not reach the graph.
func TestSpeedSamplesDropsWarmup(t *testing.T) {
	var all []Sample
	all = append(all, Sample{T: 1, Phase: "latency", RTT: 1})
	for i := 0; i < 40; i++ { // download: 10 s from t=8
		all = append(all, Sample{T: 8 + float64(i)*0.25, Mbps: 70, Phase: "download"})
	}
	all = append(all, Sample{T: 18.25, Mbps: 600, Phase: "upload"}) // the burst
	for i := 1; i < 40; i++ {
		all = append(all, Sample{T: 18.25 + float64(i)*0.25, Mbps: 100, Phase: "upload"})
	}
	all = append(all, Sample{T: 30, Mbps: 20, Phase: "udpdown"})
	got := speedSamples(all, 600)
	for _, s := range got {
		if s.Mbps > 100 {
			t.Fatalf("burst kept: %+v", s)
		}
		if s.Phase == "latency" {
			t.Fatalf("non-speed sample kept: %+v", s)
		}
	}
	if n := len(got); n != 36+36+1 { // the first second (4 samples) of each TCP phase is left out
		t.Fatalf("kept %d samples, want %d", n, 36+36+1)
	}
}
