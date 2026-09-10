package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVersionSegment(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"8", 8},
		{"09", 9},
		{"10", 10},
		{"0", 0},
		{"", 0},
		{"8rc", 8},
		{"99-beta", 99},
	}
	for _, c := range cases {
		if got := versionSegment(c.in); got != c.want {
			t.Errorf("versionSegment(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		version, target string
		want            bool
	}{
		{"2.8.0", "2.8.0", true},
		{"2.9.1", "2.8.0", true},
		{"2.10.0", "2.8.0", true},   // regression: "2.10.0" < "2.8.0" as strings
		{"2.100.0", "2.99.0", true}, // regression: string compare gets this wrong too
		{"2.7.9", "2.8.0", false},
		{"2.8", "2.8.0", true}, // equal known segments
		{"3.0.0", "2.8.0", true},
		{"01.09.03.50", "2.1.1", false},
		{"2.2.0", "2.1.1", true},
		{"2.1.0", "2.1.1", false},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.version, c.target); got != c.want {
			t.Errorf("versionAtLeast(%q, %q) = %v, want %v", c.version, c.target, got, c.want)
		}
	}
}

func TestTestUserAgent(t *testing.T) {
	forcesNopreheat := map[string]bool{
		"PrusaSlicer/2.8.0+MacOS-arm64":                  true,
		"PrusaSlicer/2.10.0+arm64 (3.10.2-202402201133)": true, // regression case
		"PrusaSlicer/2.9.3":                              true,
		"OrcaSlicer/2.1.1":                               true,
		"OrcaSlicer/2.3.0-dev":                           true,
		"PrusaSlicer/2.6.0+arm64":                        false, // too old
		"OrcaSlicer/2.1.0":                               false, // too old
		"OrcaSlicer/01.09.03.50":                         false, // legacy version scheme
		"BBL-Slicer/v01.09.03.50 (dark) Mozilla/5.0":     false, // regex does not match
		"curl/8.1.2":                                     false,
		"":                                               false,
	}
	for ua, want := range forcesNopreheat {
		apiKey := testUserAgent(ua, "mykey")
		got := strings.Contains(apiKey, "nopreheat")
		if got != want {
			t.Errorf("testUserAgent(%q) nopreheat = %v, want %v (key=%q)", ua, got, want, apiKey)
		}
	}
	// untouched key must round-trip unchanged
	if got := testUserAgent("curl/8.1.2", "abc;def;"); got != "abc;def;" {
		t.Errorf("key modified: %q", got)
	}
}

func TestArgumentsFromAPI(t *testing.T) {
	restore := func() {
		NoFix = false
		fixPreheat, fixShutoff, fixReplaceTool = true, true, true
	}
	defer restore()

	// nofix disables everything
	argumentsFromApi("something;nofix;")
	if !NoFix {
		t.Error("NoFix = false, want true")
	}

	restore()
	argumentsFromApi("nopreheat;noreplacetool;")
	if NoFix {
		t.Error("NoFix should stay false")
	}
	if fixPreheat {
		t.Error("fixPreheat = true, want false")
	}
	if !fixShutoff {
		t.Error("fixShutoff = false, want true")
	}
	if fixReplaceTool {
		t.Error("fixReplaceTool = true, want false")
	}

	restore()
	argumentsFromApi("")
	if NoFix || !fixPreheat || !fixShutoff || !fixReplaceTool {
		t.Error("empty api key must not change defaults")
	}
}

func TestStatsString(t *testing.T) {
	s := &stats{
		start:   time.Now(),
		success: 2,
		failure: 1,
		lastSuccess: &last{
			filaname: "model.gcode",
			size:     1024,
		},
		lastFailure: &last{
			filaname: "broken.gcode",
			size:     10,
		},
	}
	out := s.String()
	if !strings.Contains(out, "success: 2, failure: 1") {
		t.Errorf("stats output missing counters: %s", out)
	}
	if !strings.Contains(out, "model.gcode") {
		t.Errorf("stats output missing last success: %s", out)
	}
}

func TestWriteResponseContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeResponse(w, http.StatusOK, `{"done": true}`)
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("Code = %d", w.Code)
	}
	if w.Body.String() != `{"done": true}` {
		t.Errorf("Body = %q", w.Body.String())
	}

	// pre-set content type must be preserved
	w2 := httptest.NewRecorder()
	w2.Header().Set("Content-Type", "text/plain")
	writeResponse(w2, http.StatusOK, "hello")
	if w2.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type overwritten: %q", w2.Header().Get("Content-Type"))
	}
}
