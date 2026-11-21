package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestIsHamProgram(t *testing.T) {
	cases := []struct {
		name    string
		windows bool
		want    bool
	}{
		{"SmartSDR.exe", true, true},
		{"DAX.exe", true, true},
		{"CAT.exe", true, true},
		{"wsjtx.exe", true, true},
		{"N1MM Logger.net.exe", true, true},
		{"SDR Console.exe", true, true},
		{"fldigi", false, true},
		{"rigctld", false, true},
		{"DAX3API.exe", true, false}, // Dolby audio, on many laptops
		{"cat", false, false},        // the Linux command
		{"chrome.exe", true, false},
		{"Commander.exe", true, true},
		{"commandersvc", false, false},
		{"", true, false},
	}
	for _, c := range cases {
		if got := isHamProgram(c.name, c.windows); got != c.want {
			t.Errorf("isHamProgram(%q, windows=%v) = %v, want %v", c.name, c.windows, got, c.want)
		}
	}
}

var radioWords = regexp.MustCompile(`(?i)\bradio\b|\bSDR\b|panadapter|push-to-talk|\bCW\b|\bham\b`)

// everything a person sees for a result: report, fixes and the AI prompt
func allWords(r *Result) string {
	return r.Text() + "\n" + fixText(r) + "\n" + r.AIPrompt()
}

// Without ham software at either end, nothing a person sees mentions radio.
func TestGeneralWordingHasNoRadio(t *testing.T) {
	for name, r := range map[string]*Result{"wifi": wifiInHouse(), "sick LAN": sickLAN()} {
		r.RemoteHost = "Office-PC" // sickLAN's far end is called Radio-PC; names are shown as they are
		r.Grade("auto")
		if m := radioWords.FindAllString(allWords(r), -1); len(m) > 0 {
			t.Errorf("%s: radio words without ham software: %v", name, m)
		}
	}
}

// With ham software at either end, the radio wording comes back.
func TestHamWording(t *testing.T) {
	r := sickLAN()
	r.Ham = true
	r.Grade("auto")
	all := allWords(r)
	for _, w := range []string{"panadapter", "remote ham radio operation"} {
		if !strings.Contains(all, w) {
			t.Errorf("ham result is missing %q", w)
		}
	}
	w := wifiInHouse()
	w.Ham = true
	w.Grade("auto")
	if !strings.Contains(fixText(w), "radio computer") {
		t.Error("ham Wi-Fi advice is missing the radio wording")
	}
}

// The process list works on this computer (Windows API or /proc), and shows
// what ham radio software it finds.
func TestRunningPrograms(t *testing.T) {
	ps := runningPrograms()
	if len(ps) == 0 && isWindows {
		t.Fatal("no running programs listed")
	}
	var ham []string
	for _, p := range ps {
		if isHamProgram(p, isWindows) {
			ham = append(ham, p)
		}
	}
	t.Logf("%d programs running; ham radio software: %v", len(ps), ham)
}
