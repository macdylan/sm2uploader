package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/gosuri/uilive"
)

const (
	MoonrakerPort    = "80"
	MoonrakerTimeout = 120 // large G-code files may take a while
)

type MoonrakerConnector struct {
	printer *Printer
}

func (mc *MoonrakerConnector) Ping(p *Printer) bool {
	if !p.Moonraker {
		return false
	}
	if ping(p.IP, MoonrakerPort, 3) {
		mc.printer = p
		return true
	}
	return false
}

func (mc *MoonrakerConnector) Connect() error {
	return nil
}

func (mc *MoonrakerConnector) Disconnect() error {
	return nil
}

func (mc *MoonrakerConnector) SetToolTemperature(tool int, temperature int) error {
	return fmt.Errorf("not implemented")
}

func (mc *MoonrakerConnector) SetBedTemperature(tool int, temperature int) error {
	return fmt.Errorf("not implemented")
}

func (mc *MoonrakerConnector) Home() error {
	return fmt.Errorf("not implemented")
}

func (mc *MoonrakerConnector) Upload(payload *Payload) error {
	log.Printf("Uploading via Moonraker HTTP protocol")

	w := uilive.New()
	w.Start()
	log.SetOutput(w)
	defer func() {
		w.Stop()
		log.SetOutput(os.Stderr)
	}()

	rc, err := payload.StreamContent(NoFix)
	if err != nil {
		// G-Code fix failed, fallback to original file content
		log.SetOutput(os.Stderr)
		log.Printf("G-Code fix error(ignored): %s", err)
		log.SetOutput(w)
		return uploadMoonrakerURL(mc.URL("/server/files/upload"), payload.Name, payload.File)
	}
	defer rc.Close()

	if !NoFix && payload.ShouldBeFix() {
		log.SetOutput(os.Stderr)
		log.Printf("G-Code fixed")
		log.SetOutput(w)
	}

	return uploadMoonrakerURL(mc.URL("/server/files/upload"), payload.Name, rc)
}

// uploadMoonrakerURL is the testable core of uploadMoonraker: it spools the
// multipart/form-data body to a temporary file instead of building it in
// memory, so large G-code files never fully reside in RAM. Content-Length
// is set from the spooled size, avoiding chunked transfer encoding which
// causes 502 from nginx. A progressReader provides real-time progress.
func uploadMoonrakerURL(uploadURL, filename string, content io.Reader) error {
	spool, err := os.CreateTemp("", "sm2upload-*.multipart")
	if err != nil {
		return fmt.Errorf("moonraker create temp file failed: %w", err)
	}
	spoolPath := spool.Name()
	defer func() {
		spool.Close()
		os.Remove(spoolPath)
	}()

	mw := multipart.NewWriter(spool)
	if err := mw.WriteField("root", "gcodes"); err != nil {
		return fmt.Errorf("moonraker write field failed: %w", err)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("moonraker create form file failed: %w", err)
	}
	if _, err := io.Copy(fw, content); err != nil {
		return fmt.Errorf("moonraker write file part failed: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("moonraker finish multipart body failed: %w", err)
	}

	fi, err := spool.Stat()
	if err != nil {
		return fmt.Errorf("moonraker stat temp file failed: %w", err)
	}
	totalSize := fi.Size()

	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("moonraker rewind temp file failed: %w", err)
	}

	pr := &progressReader{
		reader:     spool,
		total:      totalSize,
		lastUpdate: time.Now(),
		onProgress: func(uploaded int64) {
			if totalSize > 0 {
				perc := float64(uploaded) / float64(totalSize) * 100.0
				log.Printf("  - Moonraker sending %.1f%%", perc)
			} else {
				log.Printf("  - Moonraker sending %s...", humanReadableSize(uploaded))
			}
		},
	}

	req, err := http.NewRequest("POST", uploadURL, pr)
	if err != nil {
		return fmt.Errorf("moonraker create request failed: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = totalSize

	client := &http.Client{
		Timeout: time.Second * time.Duration(MoonrakerTimeout),
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("moonraker upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("moonraker upload returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (mc *MoonrakerConnector) URL(path string) string {
	return fmt.Sprintf("http://%s:%s%s", mc.printer.IP, MoonrakerPort, path)
}

// progressReader wraps an io.Reader and reports progress at intervals.
type progressReader struct {
	reader     io.Reader
	total      int64
	uploaded   int64
	lastUpdate time.Time
	onProgress func(int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.uploaded += int64(n)
	if time.Since(pr.lastUpdate) >= 35*time.Millisecond {
		pr.lastUpdate = time.Now()
		pr.onProgress(pr.uploaded)
	}
	return n, err
}

func init() {
	Connector.RegisterHandler(&MoonrakerConnector{})
}
