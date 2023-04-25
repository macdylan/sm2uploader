package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/gosuri/uilive"
)

const (
	SACPPort    = "8888"
	SACPTimeout = 5
)

type SACPConnector struct {
	printer *Printer
	conn    net.Conn
}

func (sc *SACPConnector) Ping(p *Printer) bool {
	// if !p.Sacp {
	// 	return false
	// }
	if ping(p.IP, SACPPort, 3) {
		sc.printer = p
		return true

	}
	return false
}

func (sc *SACPConnector) Connect() (err error) {
	conn, err := SACP_connect(sc.printer.IP, SACPTimeout*time.Second)
	if conn != nil {
		sc.conn = conn
	}
	return err
}

func (sc *SACPConnector) Disconnect() error {
	if sc.conn != nil {
		sc.conn.Close()
	}
	return nil
}

func (sc *SACPConnector) Upload(fname string, content []byte) (err error) {
	w := uilive.New()
	w.Start()
	log.SetOutput(w)
	defer func() {
		w.Stop()
		log.SetOutput(os.Stderr)
	}()

	err = SACP_start_upload(sc.conn, fname, content, SACPTimeout*time.Second)
	return
}

func init() {
	Connector.RegisterHandler(&SACPConnector{})
}
