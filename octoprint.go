package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	maxMemory = 64 << 20 // 64MB
)

var (
	noTrim           = false
	noShutoff        = false
	noPreheat        = false
	noReinforceTower = false
	noReplaceTool    = false
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
		if len(apiKey) > 5 {
			argumentsFromApi(apiKey)
		}

		// Send the stream to the printer
		payload := NewPayload(file, fd.Filename, fd.Size)
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
	noTrim = strings.Contains(str, "notrim")
	noPreheat = strings.Contains(str, "nopreheat")
	noShutoff = strings.Contains(str, "noshutoff")
	noReinforceTower = strings.Contains(str, "noreinforcetower")
	noReplaceTool = strings.Contains(str, "noreplacetool")
	msg := []string{}
	if noTrim {
		msg = append(msg, "-notrim")
	}
	if noPreheat {
		msg = append(msg, "-nopreheat")
	}
	if noShutoff {
		msg = append(msg, "-noshutoff")
	}
	if noReinforceTower {
		msg = append(msg, "-noreinforcetower")
	}
	if noReplaceTool {
		msg = append(msg, "-noreplacetool")
	}
	if len(msg) > 0 {
		log.Printf("SMFix with args: %s", strings.Join(msg, " "))
	}
}
