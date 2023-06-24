package main

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	maxMemory = 64 << 20 // 64MB
)

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
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"api": "0.1", "server": "1.2.3", "text": "OctoPrint 1.2.3/Dummy"}`))
	})

	mux.HandleFunc("/api/files/local", func(w http.ResponseWriter, r *http.Request) {
		// Check if request is a POST request
		if r.Method != http.MethodPost {
			log.Print("Method not allowed: ", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(maxMemory); err != nil {
			log.Print("Parse form error: ", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Retrieve the uploaded file
		file, fd, err := r.FormFile("file")
		if err != nil {
			log.Print("Error retrieving file: ", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		fd.Filename = normalizedFilename(fd.Filename)

		var content []byte
		if SmFixPath != "" {
			// tmpfile for post-process
			tmpFile, err := os.CreateTemp("", "smup-*.tmp")
			if err != nil {
				log.Print("tmp file error:", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer os.Remove(tmpFile.Name())

			if _, err := io.Copy(tmpFile, file); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tmpFile.Close()

			// post process
			if err := postprocessGcodeFile(SmFixPath, tmpFile.Name()); err != nil {
				http.Error(w, "SMFix error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			content, _ = ioutil.ReadFile(tmpFile.Name())
		} else {
			content, _ = io.ReadAll(file)
		}

		// Send the stream to the printer
		if err := Connector.Upload(printer, fd.Filename, content); err != nil {
			log.Print("Error uploading file: ", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Upload finished: %s [%s]", fd.Filename, humanReadableSize(int64(len(content))))

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"done": true}`))
	})

	handler := LoggingMiddleware(mux)

	log.Printf("Starting OctoPrint server on %s ...", listenAddr)

	// Create a listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	log.Printf("Server started, now you can upload files to http://%s", listener.Addr().String())

	// Start the server
	return http.Serve(listener, handler)
}
