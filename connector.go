package main

import (
	"errors"
	"net"
	"time"
)

type connector struct {
	handlers []Handler
}

type Handler interface {
	Ping(*Printer) bool
	Connect() error
	Disconnect() error
	Upload(fname string, content []byte) error
}

func (c *connector) RegisterHandler(h Handler) {
	c.handlers = append(c.handlers, h)
}

// Upload to upload a file to a printer
func (c *connector) Upload(p *Printer, fname string, content []byte) error {
	// Iterate through all handlers
	for _, h := range c.handlers {
		// Check if handler can ping the printer
		if h.Ping(p) {
			// Connect to the printer
			if err := h.Connect(); err != nil {
				return err
			}
			// Upload the file to the printer
			if err := h.Upload(fname, content); err != nil {
				h.Disconnect()
				return err
			}
			// Disconnect from the printer
			if err := h.Disconnect(); err != nil {
				return err
			}
			// Return nil if successful
			return nil
		}
	}
	// Return error if printer is not available
	return errors.New("Printer " + p.IP + " is not available.")
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
