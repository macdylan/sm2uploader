package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/imroc/req/v3"
)

type ConnectorState int

const (
	ConnectorStateBroken ConnectorState = iota
	ConnectorStateConnecting
	ConnectorStateWaiting
	ConnectorStateConnected
	ConnectorStateUploaded
	ConnectorStatePrintingPrepared
	ConnectorStatePrinting
	ConnectorStatePrinted
	ConnectorStatePrintStopped
	ConnectorStateDisconnected
	ConnectorStateDiscovering
	ConnectorStateDiscovered
)

const (
	toolHead3DP     = 1
	toolHeadCNC     = 2
	toolHeadLaser   = 3
	toolHeadLaser10 = 4
)

type Connector struct {
	Target         *Machine
	State          chan ConnectorState
	Error          error // last error
	UploadCallback func(req.UploadInfo)
	PrintCallback  func()
	ToolHead       int
	client         *req.Client
	cancel         chan bool
}

func NewConnector() *Connector {
	client := req.C()
	client.SetCommonHeader("User-Agent", "sm2uploader/0.1")
	client.SetCommonHeader("Cache-Control", "no-cache")
	client.SetCommonHeader("Connection", "keep-alive")
	// client.EnableDumpAllWithoutRequestBody()
	// client.EnableDumpAll()

	return &Connector{
		State:          make(chan ConnectorState, 1),
		Target:         nil,
		Error:          nil,
		UploadCallback: nil,
		client:         client,
		cancel:         make(chan bool, 1),
	}
}

func (conn *Connector) setState(state ConnectorState) { conn.State <- state }

func (conn *Connector) Cancel() { conn.cancel <- true }

func (conn *Connector) broken(err any) {
	if err != nil {
		switch err.(type) {
		case string:
			conn.Error = errors.New(err.(string))
		case error:
			conn.Error = err.(error)
		}
	}
	conn.setState(ConnectorStateBroken)
}

func (conn *Connector) Discover(timeout time.Duration) <-chan *Machine {
	conn.setState(ConnectorStateDiscovering)

	ret := make(chan *Machine)

	src := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	dst := &net.UDPAddr{IP: net.ParseIP("255.255.255.255"), Port: 20054}
	c, err := net.ListenUDP("udp4", src)
	if err != nil {
		conn.broken(err)
		return ret
	}

	c.SetDeadline(time.Now().Add(timeout * time.Second))
	_, err = c.WriteToUDP([]byte("discover"), dst)
	if err != nil {
		conn.broken(err)
		return ret
	}

	go func() {
		defer close(ret)

		for {
			data := make([]byte, 512)
			n, addr, err := c.ReadFromUDP(data)
			if err != nil { // timeout
				conn.setState(ConnectorStateDiscovered)
				break
			}
			if n > 0 {
				// fmt.Printf("read %s from <%s>\n", data[:n], addr)
				machine, err := NewMachine(addr, data[:n])
				if err == nil {
					ret <- machine
				}
				// ignore invalid response
			}
		}
	}()
	return ret
}

func (conn *Connector) request(timeout ...int) *req.Request {
	t := 0
	if len(timeout) > 0 {
		t = timeout[0]
	}
	req := conn.client.SetTimeout(time.Duration(t) * time.Second).R()
	// for POST
	req.SetFormData(map[string]string{"token": conn.Target.Token})
	// for GET
	req.SetQueryParam("token", conn.Target.Token)
	// no cache
	req.SetQueryParam("_", fmt.Sprintf("%d", time.Now().Unix()))
	return req
}

func (conn *Connector) Connect() {
	conn.setState(ConnectorStateConnecting)

	api := conn.Target.URL("/connect")

	type data struct {
		Token    string `json:"token"`
		HeadType uint   `json:"headType"`
		// Readonly string `json:"readonly"`
		// Series       string `json:"series"`
		// HasEnclosure bool   `json:"hasEnclosure"`
	}
	result := &data{}
	resp, err := conn.request(3).SetResult(result).Post(api)
	if err != nil {
		conn.broken(err)
		return
	}

	if resp.IsError() {
		conn.broken(fmt.Sprintf("error code: %d from %s", resp.StatusCode, api))
		return
	}

	conn.setToken(result.Token)

	go conn.startHeartbeat()
}

func (conn *Connector) Disconnect() {
	// conn.setState(ConnectorStateDisconnecting)

	api := conn.Target.URL("/disconnect")
	resp, err := conn.request(2).Post(api)
	if err == nil && resp.IsSuccess() {
		conn.setState(ConnectorStateDisconnected)
	} else {
		conn.setState(ConnectorStateBroken)
	}
}

func (conn *Connector) heartbeat() int {
	api := conn.Target.URL("/status")

	/*
		type data struct {
			Status string `json:"status"` // IDLE/RUNNING/STOPPED/PAUSED
			// ToolHead      string `json:"toolHead"`    // TOOLHEAD_3DPRINTING_1
			// PrintStatus   string `json:"printStatus"` // Complete
			// IsFilamentOut bool   `json:"isFilamentOut"`
		}
		result := &data{}
	*/
	// if resp, err := conn.request().SetResult(result).Get(api); err == nil {

	if resp, err := conn.request(3).Get(api); err == nil {
		return resp.StatusCode
	}
	return -1
}

func (conn *Connector) startHeartbeat() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		switch conn.heartbeat() {
		case 200:
			conn.setState(ConnectorStateConnected)
		case 204:
			conn.setState(ConnectorStateWaiting)
		default:
			conn.broken("Machine was not connected")
			return
		}

		select {
		case <-ticker.C:
			continue
		case <-conn.cancel:
			return
		}
	}
}

func (conn *Connector) Upload(name string, file *os.File) {
	url := conn.Target.URL("/upload")
	r := conn.request(60)
	r.SetUploadCallback(conn.UploadCallback)
	r.SetFileReader("file", name, file)
	conn.do(r.Post, url, ConnectorStateUploaded)
}

func (conn *Connector) PreparePrint(name string, file *os.File) {
	url := conn.Target.URL("/prepare_print")
	r := conn.request(60)
	r.SetFormData(map[string]string{"type": "3DP"})
	r.SetUploadCallback(conn.UploadCallback)
	r.SetFileReader("file", name, file)
	conn.do(r.Post, url, ConnectorStatePrintingPrepared)
}

func (conn *Connector) StartPrint() {
	url := conn.Target.URL("/start_print")
	conn.do(conn.request(60).Post, url, ConnectorStatePrinting)
}

func (conn *Connector) StopPrint() {
	url := conn.Target.URL("/stop_print")
	conn.do(conn.request(5).Post, url, ConnectorStatePrintStopped)
}

func (conn *Connector) do(req func(string) (*req.Response, error), url string, state ConnectorState) {
	if resp, err := req(url); err != nil {
		conn.broken(err)
	} else if resp.IsError() {
		conn.broken(resp.Status)
	} else {
		conn.setState(state)
	}
}

func (conn *Connector) setToken(token string) { conn.Target.Token = token }

func Probe(ip string) bool {
	url := "http://" + ip + ":8080/api/v1/enclosure?token=fake"
	if resp, err := req.C().SetTimeout(2 * time.Second).R().Get(url); err == nil {
		if resp.StatusCode == 401 { // 401 UNAUTHORIZED
			return true
		}
	}
	return false
}
