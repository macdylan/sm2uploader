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
	Upload(fpath string) error
}

func (c *connector) RegisterHandler(h Handler) {
	c.handlers = append(c.handlers, h)
}

func (c *connector) Upload(p *Printer, fpath string) error {
	for _, h := range c.handlers {
		if h.Ping(p) {
			if err := h.Connect(); err != nil {
				return err
			}
			if err := h.Upload(fpath); err != nil {
				h.Disconnect()
				return err
			}
			if err := h.Disconnect(); err != nil {
				return err
			}
			return nil
		}
	}
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
