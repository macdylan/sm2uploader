package main

import (
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
	result := struct {
		Token string `json:"token"`
	}{}

	req := hc.request().
		SetResult(&result).
		SetRetryCount(3).
		SetRetryFixedInterval(1 * time.Second).
		SetRetryCondition(func(r *req.Response, err error) bool {
			if Debug {
				log.Printf("-- retrying %s -> %d, token %s", r.Request.URL, r.StatusCode, hc.printer.Token)
			}

			// token expired
			if r.StatusCode == 403 && hc.printer.Token != "" {
				hc.printer.Token = ""
				// reconnect with no token to get new one
				return true
			}
			return false
		})

	resp, err := req.Post(hc.URL("/connect"))
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
		/*
			} else if resp.StatusCode == 403 && hc.printer.Token != "" {
				// token expired
				hc.printer.Token = ""
				// reconnect with no token to get new one
				return hc.Connect()
		*/
	}

	return fmt.Errorf("connect error %d", resp.StatusCode)
}

func (hc *HTTPConnector) Disconnect() (err error) {
	if hc.client != nil && hc.printer.Token != "" {
		_, err = hc.request().Post(hc.URL("/disconnect"))
	}
	return
}

func (hc *HTTPConnector) Upload(payload *Payload) (err error) {
	finished := make(chan empty, 1)
	defer func() {
		finished <- empty{}
	}()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for {
			select {
			case <-ticker.C:
				hc.checkStatus()
			case <-finished:
				if Debug {
					log.Printf("-- heartbeat stopped")
				}
				ticker.Stop()
				return
			}
		}
	}()

	w := uilive.New()
	w.Start()
	log.SetOutput(w)
	defer func() {
		w.Stop()
		log.SetOutput(os.Stderr)
	}()

	file := req.FileUpload{
		ParamName: "file",
		FileName:  payload.Name,
		GetFileContent: func() (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				defer pw.Close()
				content, err := payload.GetContent(NoFix)
				if !NoFix {
					log.SetOutput(os.Stderr)
					if err != nil {
						log.Printf("G-Code fix error(ignored): %s", err)
					} else {
						log.Printf("G-Code fixed")
					}
					log.SetOutput(w)
				}
				pw.Write(content)
			}()
			return pr, nil
		},
		FileSize: payload.Size,
		// ContentType: "application/octet-stream",
	}
	r := hc.request(0)
	r.SetFileUpload(file)
	r.SetUploadCallbackWithInterval(func(info req.UploadInfo) {
		if info.FileSize > 0 {
			perc := float64(info.UploadedSize) / float64(info.FileSize) * 100.0
			log.Printf("  - HTTP sending %.1f%%", perc)
		} else {
			log.Printf("  - HTTP sending %s...", humanReadableSize(info.UploadedSize))
		}
	}, 35*time.Millisecond)

	_, err = r.Post(hc.URL("/upload"))
	return
}

func (hc *HTTPConnector) request(timeout ...int) *req.Request {
	to := HTTPTimeout
	if len(timeout) > 0 {
		to = timeout[0]
	}

	if hc.client == nil {
		hc.client = req.C()
		hc.client.DisableAllowGetMethodPayload()
		if Debug {
			hc.client.EnableDumpAllWithoutRequestBody()
		}
	}

	req := hc.client.SetTimeout(time.Second * time.Duration(to)).R()
	// for GET
	req.SetQueryParam("token", hc.printer.Token)
	// for POST
	req.SetFormData(map[string]string{"token": hc.printer.Token})

	return req
}

func (hc *HTTPConnector) checkStatus() (status int) {
	r, err := hc.request().Get(hc.URL("/status"))
	if Debug {
		log.Printf("-- heartbeat: %d, err(%s)", r.StatusCode, err)
	}
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
