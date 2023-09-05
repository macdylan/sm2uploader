package main

import (
	"errors"
	"io"
	"net"
	"runtime"
	"time"
)

const (
	FILE_SIZE_MIN = 1
	FILE_SIZE_MAX = 2 << 30 // 2GB
)

var (
	errFileEmpty    = errors.New("File is empty.")
	errFileTooLarge = errors.New("File is too large.")
)

type Payload struct {
	File io.Reader
	Name string
	Size int64
}

func (p *Payload) SetName(name string) {
	p.Name = normalizedFilename(name)
}

func (p *Payload) ReadableSize() string {
	return humanReadableSize(p.Size)
}

func (p *Payload) GetContent(nofix bool) (cont []byte, err error) {
	defer runtime.GC()
	if nofix || !p.ShouldBeFix() {
		cont, err = io.ReadAll(p.File)
	} else {
		cont, err = postProcess(p.File)
		p.Size = int64(len(cont))
	}
	return cont, err
}

func (p *Payload) ShouldBeFix() bool {
	return shouldBeFix(p.Name)
}

func NewPayload(file io.Reader, name string, size int64) *Payload {
	return &Payload{
		File: file,
		Name: normalizedFilename(name),
		Size: size,
	}
}

type connector struct {
	handlers []Handler
}

type Handler interface {
	Ping(*Printer) bool
	Connect() error
	Disconnect() error
	Upload(*Payload) error
}

func (c *connector) RegisterHandler(h Handler) {
	c.handlers = append(c.handlers, h)
}

// Upload to upload a file to a printer
func (c *connector) Upload(printer *Printer, payload *Payload) error {
	// Iterate through all handlers
	for _, h := range c.handlers {
		// Check if handler can ping the printer
		if h.Ping(printer) {
			// Connect to the printer
			if err := h.Connect(); err != nil {
				return err
			}
			defer h.Disconnect()

			if payload.Size > FILE_SIZE_MAX {
				return errFileTooLarge
			}
			if payload.Size < FILE_SIZE_MIN {
				return errFileEmpty
			}
			// Upload the file to the printer
			if err := h.Upload(payload); err != nil {
				return err
			}

			// Return nil if successful
			return nil
		}
	}
	// Return error if printer is not available
	return errors.New("Printer " + printer.IP + " is not available.")
}

var Connector = &connector{}

// ping the printer to see if it is available
func ping(ip string, port string, timeout int) bool {
	if timeout <= 0 {
		timeout = 2
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), time.Second*time.Duration(timeout))
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
