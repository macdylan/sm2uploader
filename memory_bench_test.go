package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// allocDuring runs fn and returns the total bytes allocated while it ran.
// TotalAlloc is cumulative, so this is immune to GC timing.
func allocDuring(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestStreamContentBoundedMemory verifies that streaming a large file in
// nofix mode never loads it into memory (regression guard for the
// RAM-constrained ARM target).
func TestStreamContentBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large-file test in short mode")
	}
	const size = 200 << 20 // 200MB

	path := filepath.Join(t.TempDir(), "big.gcode")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// sparse file: instant creation, no 200MB written
	if _, err := f.Seek(size-1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0x0A}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	rf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	NoFixOrig := NoFix
	NoFix = true
	defer func() { NoFix = NoFixOrig }()

	var n int64
	var streamErr error
	alloc := allocDuring(func() {
		p := NewPayload(rf, "big.gcode", size)
		rc, err := p.StreamContent(true)
		if err != nil {
			streamErr = err
			return
		}
		defer rc.Close()
		n, streamErr = io.Copy(io.Discard, rc)
	})
	if streamErr != nil {
		t.Fatalf("stream: %v", streamErr)
	}
	if n != size {
		t.Fatalf("streamed %d bytes, want %d", n, size)
	}
	const limit = 64 << 20
	if alloc > limit {
		t.Errorf("streaming 200MB allocated %d bytes (limit %d)", alloc, limit)
	}
}

// TestMoonrakerUploadBoundedMemory verifies the multipart spool path keeps
// memory bounded and still sends Content-Length (no chunked encoding).
func TestMoonrakerUploadBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping upload test in short mode")
	}
	const size = 5 << 20
	content := bytes.Repeat([]byte("G1 X1 Y1 F6000\n"), size/15+1)[:size]

	var gotLength int64
	var gotChunked bool
	var gotFileContent []byte
	var gotChecksumValid bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLength = r.ContentLength
		gotChunked = r.TransferEncoding != nil && len(r.TransferEncoding) > 0
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("server: parse multipart: %v", err)
			w.WriteHeader(400)
			return
		}
		if root := r.FormValue("root"); root != "gcodes" {
			t.Errorf("server: root = %q, want gcodes", root)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("server: form file: %v", err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		gotFileContent, _ = io.ReadAll(file)
		sum := sha256.Sum256(gotFileContent)
		gotChecksumValid = hex.EncodeToString(sum[:]) == r.FormValue("checksum")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	uploadErr := error(nil)
	alloc := allocDuring(func() {
		uploadErr = uploadMoonrakerURL(srv.URL+"/server/files/upload", "model.gcode", bytes.NewReader(content))
	})
	if uploadErr != nil {
		t.Fatalf("uploadMoonraker: %v", uploadErr)
	}
	if gotLength <= 0 {
		t.Errorf("ContentLength = %d, want > 0 (nginx 502 guard)", gotLength)
	}
	if gotChunked {
		t.Error("request used chunked transfer encoding")
	}
	if !bytes.Equal(gotFileContent, content) {
		t.Errorf("server received %d bytes, want %d", len(gotFileContent), len(content))
	}
	if !gotChecksumValid {
		t.Error("checksum field missing or does not match file content SHA256")
	}

	const limit = 64 << 20
	if alloc > limit {
		t.Errorf("upload allocated %d bytes (limit %d)", alloc, limit)
	}
}

// TestMoonrakerUploadErrorStatus ensures non-201 responses surface as errors.
func TestMoonrakerUploadErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	err := uploadMoonrakerURL(srv.URL+"/server/files/upload", "model.gcode", strings.NewReader("G1"))
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("err = %v, want HTTP 502 error", err)
	}
}

func BenchmarkPostProcessMemory(b *testing.B) {
	// valid metadata header + ~64MB of realistic multi-tool g-code
	var buf bytes.Buffer
	buf.WriteString(sampleGcode)
	toolLines := []string{
		"T0\n", "M104 S210\n", "G1 X100.5 Y100.2 E1.5 F3000\n",
		"G1 X101.5 Y101.2 E2.5 F3000\n", "G4 S0\n", "; comment line\n",
		"T1\n", "M104 S200\n", "G1 X102.5 Y102.2 E3.5 F3000\n",
	}
	target := int64(64 << 20)
	for int64(buf.Len()) < target {
		for _, l := range toolLines {
			buf.WriteString(l)
		}
	}
	data := buf.Bytes()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	var peak uint64
	for i := 0; i < b.N; i++ {
		var peakHere uint64
		alloc := allocDuring(func() {
			if _, err := postProcess(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		})
		peakHere = alloc
		if peakHere > peak {
			peak = peakHere
		}
	}
	b.StopTimer()
	mb := float64(peak) / (1024 * 1024)
	b.ReportMetric(mb, "peakAllocMB")
	fmt.Fprintf(os.Stderr, "\npostProcess peak allocation: %.1f MB for %.1f MB input (%.1fx)\n",
		mb, float64(len(data))/(1024*1024), mb/(float64(len(data))/(1024*1024)))
}
