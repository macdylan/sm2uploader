package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeHandler struct {
	name      string
	pingOK    bool
	connectOK bool
	uploadErr error
	uploaded  int
	connected int
}

func (f *fakeHandler) Ping(p *Printer) bool { return f.pingOK }
func (f *fakeHandler) Connect() error {
	if !f.connectOK {
		return errors.New("connect failed")
	}
	f.connected++
	return nil
}
func (f *fakeHandler) Disconnect() error { return nil }
func (f *fakeHandler) Upload(payload *Payload) error {
	f.uploaded++
	return f.uploadErr
}
func (f *fakeHandler) SetToolTemperature(int, int) error { return nil }
func (f *fakeHandler) SetBedTemperature(int, int) error  { return nil }
func (f *fakeHandler) Home() error                       { return nil }

func TestPayloadNameNormalization(t *testing.T) {
	p := NewPayload(strings.NewReader("x"), "/tmp/dir/model.gcode", 1)
	if p.Name != "tmp/dir/model.gcode" {
		t.Errorf("Name = %q", p.Name)
	}
	p.SetName("/other/file.nc")
	if p.Name != "other/file.nc" {
		t.Errorf("SetName = %q", p.Name)
	}
}

func TestStreamContentNoFixPassthrough(t *testing.T) {
	NoFixOrig := NoFix
	NoFix = true
	defer func() { NoFix = NoFixOrig }()

	content := "raw g-code content"
	p := NewPayload(strings.NewReader(content), "model.gcode", int64(len(content)))

	rc, err := p.StreamContent(true)
	if err != nil {
		t.Fatalf("StreamContent: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestStreamContentFixedFile(t *testing.T) {
	NoFixOrig := NoFix
	NoFix = false
	defer func() { NoFix = NoFixOrig }()

	dir := t.TempDir()
	fixedPath := filepath.Join(dir, "model_fixed.gcode")
	fixedContent := "fixed content from disk"
	if err := os.WriteFile(fixedPath, []byte(fixedContent), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewPayload(strings.NewReader("original"), "model.gcode", 8)
	p.FixedFile = fixedPath

	rc, err := p.StreamContent(false)
	if err != nil {
		t.Fatalf("StreamContent: %v", err)
	}
	defer rc.Close()
	if p.Size != int64(len(fixedContent)) {
		t.Errorf("Size = %d, want %d (updated from disk)", p.Size, len(fixedContent))
	}
	got, _ := io.ReadAll(rc)
	if string(got) != fixedContent {
		t.Errorf("content = %q, want %q", got, fixedContent)
	}
}

func TestStreamContentPipe(t *testing.T) {
	NoFixOrig := NoFix
	NoFix = false
	defer func() { NoFix = NoFixOrig }()

	input := sampleGcode
	p := NewPayload(strings.NewReader(input), "model.gcode", int64(len(input)))

	rc, err := p.StreamContent(false)
	if err != nil {
		t.Fatalf("StreamContent: %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want, err := postProcess(strings.NewReader(input))
	if err != nil {
		t.Fatalf("postProcess: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("pipe output differs from postProcess output")
	}
}

func TestStreamContentMissingFixedFile(t *testing.T) {
	p := NewPayload(strings.NewReader("x"), "model.gcode", 1)
	p.FixedFile = filepath.Join(t.TempDir(), "does-not-exist.gcode")
	if _, err := p.StreamContent(false); err == nil {
		t.Error("expected error for missing FixedFile")
	}
}

func TestConnectorUploadSelectsPingingHandler(t *testing.T) {
	c := &connector{}
	no := &fakeHandler{name: "no", pingOK: false}
	yes := &fakeHandler{name: "yes", pingOK: true, connectOK: true}
	c.RegisterHandler(no)
	c.RegisterHandler(yes)

	p := NewPayload(strings.NewReader("data"), "model.gcode", 4)
	if err := c.Upload(&Printer{IP: "1.2.3.4"}, p); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if yes.uploaded != 1 {
		t.Error("pinging handler was not used")
	}
	if no.uploaded != 0 {
		t.Error("non-pinging handler was used")
	}
}

func TestConnectorUploadNoHandlerAvailable(t *testing.T) {
	c := &connector{}
	c.RegisterHandler(&fakeHandler{name: "no", pingOK: false})

	p := NewPayload(strings.NewReader("data"), "model.gcode", 4)
	err := c.Upload(&Printer{IP: "1.2.3.4"}, p)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("err = %v, want 'not available'", err)
	}
}

func TestConnectorUploadSizeGuards(t *testing.T) {
	c := &connector{}
	h := &fakeHandler{name: "h", pingOK: true, connectOK: true}
	c.RegisterHandler(h)

	printer := &Printer{IP: "1.2.3.4"}

	tooBig := NewPayload(strings.NewReader("x"), "model.gcode", FILE_SIZE_MAX+1)
	if err := c.Upload(printer, tooBig); err != errFileTooLarge {
		t.Errorf("err = %v, want errFileTooLarge", err)
	}

	empty := NewPayload(strings.NewReader(""), "model.gcode", 0)
	if err := c.Upload(printer, empty); err != errFileEmpty {
		t.Errorf("err = %v, want errFileEmpty", err)
	}

	if h.uploaded != 0 {
		t.Error("Upload should have been rejected before reaching the handler")
	}
}

func TestConnectorUploadConnectFailure(t *testing.T) {
	c := &connector{}
	c.RegisterHandler(&fakeHandler{name: "h", pingOK: true, connectOK: false})

	p := NewPayload(strings.NewReader("data"), "model.gcode", 4)
	err := c.Upload(&Printer{IP: "1.2.3.4"}, p)
	if err == nil || !strings.Contains(err.Error(), "connect failed") {
		t.Errorf("err = %v, want connect failure", err)
	}
}

func TestConnectorPreHeatCommands(t *testing.T) {
	c := &connector{}
	h := &fakeHandler{name: "h", pingOK: true, connectOK: true}
	c.RegisterHandler(h)

	printer := &Printer{IP: "1.2.3.4"}
	if err := c.PreHeatCommands(printer, 200, 0, 60, false); err != nil {
		t.Fatalf("PreHeatCommands: %v", err)
	}
	if h.connected != 1 {
		t.Errorf("connected = %d, want 1", h.connected)
	}
}

func TestPingUnavailablePort(t *testing.T) {
	// nothing should listen on the TCP echo port in the test sandbox
	if ping("127.0.0.1", "1", 1) {
		t.Error("ping to closed port succeeded")
	}
}
