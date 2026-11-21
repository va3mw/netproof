package main

// Is this a ham radio station? NetProof is used by radio operators and by
// anyone with real-time audio or VoIP trouble. Results speak in general terms
// (live audio, VoIP, streaming) unless ham radio software is running on
// either computer; then they can talk about radio audio, panadapters, SDR
// data, CW and push-to-talk.
//
// Detection only reads the list of running programs (Windows API, /proc on
// Linux). It never starts anything. Not available on macOS yet.

import (
	"strings"
	"sync"
	"time"
)

// hamPrograms are lower-case program names without ".exe" or spaces; a
// running program matches when its name starts with one of them.
var hamPrograms = []string{
	// FlexRadio
	"smartsdr", "flexradio", "frstack", "slicemaster",
	// digital modes and logging
	"wsjtx", "jt9", "jtdx", "js8call", "jtalert", "gridtracker", "fldigi", "flrig", "flmsg",
	"n1mm", "log4om", "hamradiodeluxe", "hrdlogbook", "dm780", "dxlab", "winwarbler",
	"varac", "vara", "rmsexpress", "winlink", "direwolf", "wfview",
	// SDR and rig control
	"sdrconsole", "sdruno", "sdrsharp", "thetis", "powersdr", "expertsdr", "hdsdr",
	"omnirig", "rigctld", "rigctl", "remoterig",
	// Michael's tools for Flex
	"slicetci", "sliceskimmer", "sliceaprs", "flexwspr", "antennadeck", "fmtassistant", "flexreflab",
}

// Names too short or common to match by prefix; matched exactly, Windows only.
// ("dax" by prefix would also catch Dolby's DAX3API.exe.)
var hamExactWindows = []string{"cat", "dax", "commander"} // SmartSDR CAT and DAX, DXLab Commander

func isHamProgram(name string, windows bool) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".exe")
	n = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(n)
	if n == "" {
		return false
	}
	if windows {
		for _, x := range hamExactWindows {
			if n == x {
				return true
			}
		}
	}
	for _, p := range hamPrograms {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

var hamCache struct {
	sync.Mutex
	at  time.Time
	ham bool
}

// hamStation reports whether ham radio software is running here, checked at
// most every 30 seconds.
func hamStation() bool {
	hamCache.Lock()
	defer hamCache.Unlock()
	if time.Since(hamCache.at) > 30*time.Second {
		hamCache.ham = false
		for _, p := range runningPrograms() {
			if isHamProgram(p, isWindows) {
				hamCache.ham = true
				break
			}
		}
		hamCache.at = time.Now()
	}
	return hamCache.ham
}

// say picks the wording for this result: general, or ham radio terms when
// either computer runs ham radio software.
func (r *Result) say(general, ham string) string { return sayHam(r.Ham, general, ham) }

func sayHam(isHam bool, general, ham string) string {
	if isHam {
		return ham
	}
	return general
}
