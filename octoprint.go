package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	maxMemory = 256 << 20 // 256MB
)

var (
	fixShutoff        = true
	fixPreheat        = true
	// fixReinforceTower = true
	fixReplaceTool    = true

	// userAgent: OrcaSlicer/01.09.03.50
	// userAgent: BBL-Slicer/v01.09.03.50 (dark) Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)
	// userAgent: PrusaSlicer/2.6.0+arm64 (3.10.2-202402201133)
	// userAgent: PrusaSlicer/2.8.0+MacOS-arm64
	reUserAgent = regexp.MustCompile(`^(\w+)/(\S+)(?:[+-].*)?$`)
)

type stats struct {
	start       time.Time
	memory      uint64
	success     uint
	failure     uint
	lastSuccess *last
	lastFailure *last
}

type last struct {
	filaname string
	size     int64
	time     time.Time
}

func (s *stats) addSuccess(filaname string, size int64) {
	s.success++
	s.lastSuccess = &last{
		filaname: normalizedFilename(filaname),
		size:     size,
		time:     time.Now(),
	}
}

func (s *stats) addFailure(filaname string, size int64) {
	s.failure++
	s.lastFailure = &last{
		filaname: normalizedFilename(filaname),
		size:     size,
		time:     time.Now(),
	}
}

func (s *stats) String() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	s.memory = mem.Alloc

	buf := bytes.Buffer{}
	buf.WriteString("memory alloc: " + humanReadableSize(int64(s.memory)) + "\n")
	buf.WriteString("uptime: " + time.Since(s.start).String() + "\n")
	buf.WriteString(fmt.Sprintf("success: %d, failure: %d\n", s.success, s.failure))
	buf.WriteString(fmt.Sprintf("last success: %s\n - %s (%s)\n", s.lastSuccess.time.Format(time.RFC3339), s.lastSuccess.filaname, humanReadableSize(s.lastSuccess.size)))
	buf.WriteString(fmt.Sprintf("last failure: %s\n - %s (%s)\n", s.lastFailure.time.Format(time.RFC3339), s.lastFailure.filaname, humanReadableSize(s.lastFailure.size)))
	return buf.String()
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Printf("Request %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
		}()
		next.ServeHTTP(w, r)
	})
}

func startOctoPrintServer(listenAddr string, printer *Printer) error {
	var (
		_stats *stats
		mux    = http.NewServeMux()
	)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		protocol := "HTTP"
		if printer.Sacp {
			protocol = "SACP"
		} else if printer.Moonraker {
			protocol = "Moonraker"
		}
		resp := `sm2uploader ` + Version + ` - https://github.com/macdylan/sm2uploader` + "\n\n" +
			`	printer id: ` + printer.ID + "\n" +
			`	printer ip: ` + printer.IP + "\n" +
			`	protocol: ` + protocol + "\n\n" +
			_stats.String()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writeResponse(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		respVersion := `{"api": "0.1", "server": "1.2.3", "text": "OctoPrint 1.2.3/Dummy"}`
		writeResponse(w, http.StatusOK, respVersion)
	})

	mux.HandleFunc("/api/files/local", func(w http.ResponseWriter, r *http.Request) {
		// Check if request is a POST request
		if r.Method != http.MethodPost {
			methodNotAllowedResponse(w, r.Method)
			return
		}

		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			internalServerErrorResponse(w, err.Error())
			return
		}

		// Retrieve the uploaded file
		file, fd, err := r.FormFile("file")
		if err != nil {
			bedRequestResponse(w, err.Error())
			return
		}
		defer file.Close()

		// read X-Api-Key header
		apiKey := r.Header.Get("X-Api-Key")
		apiKey = testUserAgent(r.Header.Get("User-Agent"), apiKey)
		if len(apiKey) > 5 {
			argumentsFromApi(apiKey)
		}

		// Send the stream to the printer
		payload := NewPayload(file, fd.Filename, fd.Size)

		// Moonraker/Klipper devices don't need G-Code fix
		moonrakerNoFix := printer.Moonraker
		effectiveNoFix := NoFix || moonrakerNoFix
		if moonrakerNoFix && !NoFix {
			log.Printf("Moonraker device detected, skipping G-Code fix for '%s'", payload.Name)
		}

		// If output directory is specified and the file needs fixing,
		// pre-process it and save both original and fixed files to disk.
		if OutputDir != "" && payload.ShouldBeFix() && !effectiveNoFix {
			origContent, readErr := io.ReadAll(file)
			if readErr != nil {
				log.Printf("Warning: failed to read '%s' for output: %s", payload.Name, readErr)
			} else {
				fixedContent, procErr := postProcess(bytes.NewReader(origContent))
				if procErr != nil {
					log.Printf("Warning: failed to post-process '%s' for output: %s", payload.Name, procErr)
				} else {
					fixedPath, saveErr := saveToOutputDir(payload.Name, bytes.NewReader(origContent), fixedContent, true)
					if saveErr != nil {
						log.Printf("Warning: failed to save to output dir: %s", saveErr)
					} else if fixedPath != "" {
						payload.FixedFile = fixedPath
						payload.Size = int64(len(fixedContent))
						log.Printf("Saved: original -> %s/%s, fixed -> %s/%s_fixed%s",
							OutputDir, payload.Name, OutputDir, payload.Name[:len(payload.Name)-len(filepath.Ext(payload.Name))], filepath.Ext(payload.Name))
					}
				}
			}
		} else if OutputDir != "" {
			log.Printf("Skipping output save for '%s' (shouldFix=%v, nofix=%v)",
				payload.Name, payload.ShouldBeFix(), effectiveNoFix)
		}

		if err := Connector.Upload(printer, payload); err != nil {
			_stats.addFailure(payload.Name, payload.Size)
			internalServerErrorResponse(w, err.Error())
			return
		}

		_stats.addSuccess(payload.Name, payload.Size)

		log.Printf("Upload finished: %s [%s]", fd.Filename, payload.ReadableSize())

		// Return success response
		writeResponse(w, http.StatusOK, `{"done": true}`)
	})

	handler := LoggingMiddleware(mux)
	log.Printf("Starting OctoPrint server on %s ...", listenAddr)

	// Create a listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	_stats = &stats{
		start:   time.Now(),
		success: 0,
		failure: 0,
		lastSuccess: &last{
			filaname: "",
			size:     0,
			time:     time.Now(),
		},
		lastFailure: &last{
			filaname: "",
			size:     0,
			time:     time.Now(),
		},
	}

	log.Printf("Server started, now you can upload files to http://%s", listener.Addr().String())
	// Start the server
	return http.Serve(listener, handler)
}

func writeResponse(w http.ResponseWriter, status int, body string) {
	if has := w.Header().Get("Content-Type"); has == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func methodNotAllowedResponse(w http.ResponseWriter, method string) {
	log.Print("Method not allowed: ", method)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func internalServerErrorResponse(w http.ResponseWriter, err string) {
	log.Print("Internal server error: ", err)
	http.Error(w, err, http.StatusInternalServerError)
}

func bedRequestResponse(w http.ResponseWriter, err string) {
	log.Print("Bad request: ", err)
	http.Error(w, err, http.StatusBadRequest)
}

func argumentsFromApi(str string) {
	if strings.TrimSpace(str) == "" {
		return
	}
	if strings.Contains(str, "nofix") {
		NoFix = true
		log.Printf("SMFix disabled via API key (nofix)")
		return
	}
	fixPreheat = !strings.Contains(str, "nopreheat")
	fixShutoff = !strings.Contains(str, "noshutoff")
	// fixReinforceTower = !strings.Contains(str, "noreinforcetower")
	fixReplaceTool = !strings.Contains(str, "noreplacetool")

	msg := []string{}
	if fixPreheat {
		msg = append(msg, "-preheat")
	} else {
		msg = append(msg, "-nopreheat")
	}
	if fixShutoff {
		msg = append(msg, "-shutoff")
	} else {
		msg = append(msg, "-noshutoff")
	}
	// if fixReinforceTower {
	// 	msg = append(msg, "-reinforcetower")
	// } else {
	// 	msg = append(msg, "-noreinforcetower")
	// }
	if fixReplaceTool {
		msg = append(msg, "-replacetool")
	} else {
		msg = append(msg, "-noreplacetool")
	}
	if len(msg) > 0 {
		log.Printf("SMFix with args: %s", strings.Join(msg, " "))
	}
}

func testUserAgent(userAgent, apiKey string) string {
	matches := reUserAgent.FindStringSubmatch(userAgent)
	if len(matches) >= 2 {
		slicerName := matches[1]
		slicerVersion := matches[2]
		if (slicerName == "PrusaSlicer" && slicerVersion >= "2.8.0") || (slicerName == "OrcaSlicer" && slicerVersion >= "2.1.1") {
			if !strings.Contains(apiKey, "nopreheat") && strings.Contains(apiKey, "preheat") {
				apiKey = strings.Replace(apiKey, "preheat", "nopreheat", -1)
			} else {
				apiKey += ";nopreheat;"
			}
		// if !strings.Contains(apiKey, "noreinforcetower") && strings.Contains(apiKey, "reinforceTower") {
		// 	apiKey = strings.Replace(apiKey, "reinforceTower", "noreinforcetower", -1)
		// } else {
		// 	apiKey += ";noreinforcetower;"
		// }
		}
	}
	return apiKey
}
