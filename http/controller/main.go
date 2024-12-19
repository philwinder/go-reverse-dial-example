package main

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	// Start an HTTP server
	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Runner connected")

		// Hijack the connection
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Server does not support connection hijacking", http.StatusInternalServerError)
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		log.Println("Connection hijacked. Sending requests to runner...")

		// Use the hijacked connection to send HTTP requests to the runner
		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		for {
			// Create a simple HTTP request
			req, err := http.NewRequest("GET", "/status", nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				return
			}

			// Write the request to the hijacked connection
			if err := req.Write(writer); err != nil {
				log.Printf("Failed to write request: %v", err)
				return
			}
			writer.Flush()

			// Read the response from the runner
			resp, err := http.ReadResponse(reader, req)
			if err != nil {
				log.Printf("Failed to read response: %v", err)
				return
			}
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read response body: %v", err)
				return
			}

			log.Printf("Response from runner: %s, %s", resp.Status, string(respBody))
			resp.Body.Close()

			// Sleep before sending the next request
			time.Sleep(5 * time.Second)
		}
	})

	log.Println("Controller listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
