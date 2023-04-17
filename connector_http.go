package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gosuri/uilive"
	"github.com/imroc/req/v3"
)

const (
	HTTPPort    = "8080"
	HTTPTimeout = 5
)

const (
	AuthStatusApproved = 1 + iota
	AuthStatusDenied
	AuthStatusWaiting
)

type HTTPConnector struct {
	client  *req.Client
	printer *Printer
}

func (hc *HTTPConnector) Ping(p *Printer) bool {
	if p.Sacp {
		return false
	}
	if ping(p.IP, HTTPPort, 3) {
		hc.printer = p
		return true
	}
	return false
}

func (hc *HTTPConnector) Connect() error {
	hc.client = req.C()
	hc.client.DisableAllowGetMethodPayload()
	// hc.client.EnableDumpAllWithoutRequestBody()

	type resp_data struct {
		Token string `json:"token"`
	}
	result := &resp_data{}
	resp, err := hc.request().SetSuccessResult(result).Post(hc.URL("/connect"))
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		if hc.printer.Token != result.Token {
			hc.printer.Token = result.Token
		}
		tip := false
		for {
			switch hc.checkStatus() {
			case AuthStatusApproved:
				return nil
			case AuthStatusWaiting:
				if !tip {
					tip = true
					log.Println(">>> Please tap Yes on Snapmaker touchscreen to continue <<<")
				}
				// wait for auth on HMI
				<-time.After(2 * time.Second)
			case AuthStatusDenied:
				return fmt.Errorf("access denied")
			}
		}
	} else if resp.StatusCode == 403 && hc.printer.Token != "" {
		// token expired
		hc.printer.Token = ""
		// reconnect with no token to get new one
		return hc.Connect()
	}

	return fmt.Errorf("connect error %d", resp.StatusCode)
}

func (hc *HTTPConnector) Disconnect() (err error) {
	if hc.printer.Token != "" {
		_, err = hc.request().Post(hc.URL("/disconnect"))
	}
	return
}

func (hc *HTTPConnector) Upload(fname string, content []byte) (err error) {
	finished := make(chan bool, 1)
	defer func() {
		finished <- true
	}()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				hc.checkStatus()
			case <-finished:
				ticker.Stop()
				return
			}
		}
	}()

	file := req.FileUpload{
		ParamName: "file",
		FileName:  fname,
		GetFileContent: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(content)), nil
		},
		FileSize: int64(len(content)),
	}

	w := uilive.New()
	w.Start()
	defer w.Stop()
	req := hc.request(0).SetFileUpload(file).SetUploadCallback(func(info req.UploadInfo) {
		log.SetOutput(w)
		defer log.SetOutput(os.Stderr)

		perc := float64(info.UploadedSize) / float64(info.FileSize) * 100.0
		log.Printf("  - HTTP sending %.1f%%", perc)
	})

	// disable chucked to make Content-Length
	// req.DisableForceChunkedEncoding()
	_, err = req.Post(hc.URL("/upload"))
	return
}

func (hc *HTTPConnector) request(timeout ...int) *req.Request {
	to := HTTPTimeout
	if len(timeout) > 0 {
		to = timeout[0]
	}
	req := hc.client.SetTimeout(time.Second * time.Duration(to)).R()
	// for POST
	req.SetFormData(map[string]string{"token": hc.printer.Token})
	// for GET
	req.SetQueryParam("token", hc.printer.Token)
	// no cache
	req.SetQueryParam("_", fmt.Sprintf("%d", time.Now().Unix()))
	return req
}

func (hc *HTTPConnector) checkStatus() (status int) {
	r, err := hc.request().Get(hc.URL("/status"))
	if err == nil {
		switch r.StatusCode {
		case 200:
			return AuthStatusApproved
		case 204:
			return AuthStatusWaiting
			// case 401:
			// 	return AuthStatusDenied
			// case 403:
			// 	if hc.printer.Token != "" { hc.printer.Token = ""}
			// 	return AuthStatusExpired
		}
	}
	return AuthStatusDenied
}

/*
URL to make url with path
*/
func (hc *HTTPConnector) URL(path string) string {
	return fmt.Sprintf("http://%s:%s/api/v1%s", hc.printer.IP, HTTPPort, path)
}

func init() {
	Connector.RegisterHandler(&HTTPConnector{})
}
