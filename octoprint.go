package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// Log the request
		log.Printf("Request %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func startOctoPrintServer(listenAddr string, printer *Printer) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"api": "0.1", "server": "1.2.3", "text": "OctoPrint 1.2.3/Dummy"}`))
	})

	mux.HandleFunc("/api/files/local", func(w http.ResponseWriter, r *http.Request) {
		// Check if request is a POST request
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Retrieve the uploaded file
		file, fd, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		fd.Filename = normalizedFilename(fd.Filename)

		// Send the stream to the printer
		content, _ := io.ReadAll(file)
		if err := Connector.Upload(printer, fd.Filename, content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Upload finished: %s [%s]", fd.Filename, humanReadableSize(int64(len(content))))

		// Return success response
		w.WriteHeader(http.StatusOK)
	})

	handler := LoggingMiddleware(mux)

	log.Printf("Starting OctoPrint server on %s ...", listenAddr)

	// Create a listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	log.Printf("Server started, now you can upload files to http://localhost:%s", listenAddr)

	// Start the server
	return http.Serve(listener, handler)
}
