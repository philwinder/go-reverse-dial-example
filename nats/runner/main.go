package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
)

type Task struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
}

type TaskResponse struct {
	TaskID     int    `json:"task_id"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

func main() {
	// Connect to the embedded NATS server
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Error connecting to NATS server: %v", err)
	}
	defer nc.Close()

	// Start the HTTP server for task handlers
	go func() {
		setupRoutes()
		log.Println("Starting HTTP server on http://localhost:9000")
		if err := http.ListenAndServe(":9000", nil); err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	// Subscribe to the "tasks" subject to receive tasks
	_, err = nc.Subscribe("tasks", func(msg *nats.Msg) {
		var task Task
		if err := json.Unmarshal(msg.Data, &task); err != nil {
			log.Printf("Error unmarshalling task: %v", err)
			return
		}

		log.Printf("Received Task %d: %s %s", task.ID, task.Method, task.URL)

		// Execute the task via an HTTP handler
		response := executeTaskViaHTTP(task)

		// Respond directly to the request
		data, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
			return
		}

		if err := msg.Respond(data); err != nil {
			log.Printf("Error responding to task %d: %v", task.ID, err)
		} else {
			log.Printf("Sent response for Task %d", task.ID)
		}
	})
	if err != nil {
		log.Fatalf("Error subscribing to tasks: %v", err)
	}

	// Keep the runner running
	select {}
}

// Set up HTTP routes for the runner
func setupRoutes() {
	http.HandleFunc("/example", func(w http.ResponseWriter, r *http.Request) {
		body := new(strings.Builder)
		_, _ = io.Copy(body, r.Body)
		response := fmt.Sprintf("Processed example task with body: %s", body.String())
		w.Write([]byte(response))
	})
}

// Execute a task via the runner's HTTP handler
func executeTaskViaHTTP(task Task) TaskResponse {
	req, err := http.NewRequest(task.Method, "http://localhost:9000"+task.URL, bytes.NewBuffer([]byte(task.Body)))
	if err != nil {
		log.Printf("Error creating HTTP request for Task %d: %v", task.ID, err)
		return TaskResponse{TaskID: task.ID, StatusCode: 500, Body: "Internal Error"}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error executing HTTP handler for Task %d: %v", task.ID, err)
		return TaskResponse{TaskID: task.ID, StatusCode: 500, Body: "Internal Error"}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return TaskResponse{
		TaskID:     task.ID,
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}
