package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
)

func main() {
	controllerAddress := "localhost:8080"

	// Create a context that listens for OS signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture OS signals to handle graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		log.Println("Received shutdown signal. Exiting...")
		cancel()
	}()

	// Dial the controller
	conn, err := net.Dial("tcp", controllerAddress)
	if err != nil {
		log.Fatalf("Failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Send an initial HTTP request to establish the connection
	req, err := http.NewRequest("POST", "http://"+controllerAddress+"/connect", nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	if err := req.Write(conn); err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	log.Println("Connected to controller. Waiting for requests...")

	// Create a new Gorilla Mux router
	router := mux.NewRouter()
	router.HandleFunc("/status", statusHandler).Methods("GET")
	router.HandleFunc("/data", dataHandler).Methods("GET")
	router.HandleFunc("/custom/{param}", customHandler).Methods("GET")

	// Start serving HTTP requests over the hijacked connection
	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done():
			// Context was canceled, exit the loop
			log.Println("Shutting down runner gracefully...")
			return
		default:
			// Process incoming requests
			req, err := http.ReadRequest(reader)
			if err != nil {
				log.Printf("Connection closed or error reading request: %v", err)
				// Return from the loop when connection is broken
				return
			}

			// Use a ResponseWriter implementation to write the response
			writer := &connectionResponseWriter{
				conn: conn,
			}

			// Serve the request using the Gorilla Mux router
			router.ServeHTTP(writer, req)
		}
	}
}

// Handlers for different routes

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Runner is alive"))
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Here is some data from the runner"))
}

func customHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	param := vars["param"]
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("You requested parameter: " + param))
}

type connectionResponseWriter struct {
	conn       net.Conn
	header     http.Header
	statusCode int
	written    bool
}

func (w *connectionResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *connectionResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.statusCode = statusCode
}

func (w *connectionResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.statusCode = http.StatusOK
	}

	// Create the complete response
	resp := http.Response{
		StatusCode:    w.statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        w.Header(),
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
	}

	// Write the complete response at once
	err := resp.Write(w.conn)
	if err != nil {
		return 0, err
	}

	w.written = true
	return len(data), nil
}
