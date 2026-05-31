package main

import (
	"bytes"
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
	httpClient *http.Client
	printer    *Printer
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
		fileContent, readErr := io.ReadAll(payload.File)
		if readErr != nil {
			return fmt.Errorf("moonraker read content failed: %w", readErr)
		}
		return uploadMoonraker(mc, payload.Name, fileContent)
	}
	defer rc.Close()

	fileContent, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("moonraker read content failed: %w", err)
	}

	if !NoFix && payload.ShouldBeFix() {
		log.SetOutput(os.Stderr)
		log.Printf("G-Code fixed")
		log.SetOutput(w)
	}

	return uploadMoonraker(mc, payload.Name, fileContent)
}

// uploadMoonraker builds the full multipart/form-data body in memory so
// Content-Length is set, avoiding chunked transfer encoding which causes
// 502 from nginx. A progressReader provides real-time upload progress.
func uploadMoonraker(mc *MoonrakerConnector, filename string, fileContent []byte) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("root", "gcodes")
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("moonraker create form file failed: %w", err)
	}
	if _, err := fw.Write(fileContent); err != nil {
		return fmt.Errorf("moonraker write file part failed: %w", err)
	}
	mw.Close()

	totalSize := int64(buf.Len())

	pr := &progressReader{
		reader:     &buf,
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

	req, err := http.NewRequest("POST", mc.URL("/server/files/upload"), pr)
	if err != nil {
		return fmt.Errorf("moonraker create request failed: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = totalSize

	client := mc.httpClient
	if client == nil {
		client = &http.Client{
			Timeout: time.Second * time.Duration(MoonrakerTimeout),
		}
		mc.httpClient = client
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
