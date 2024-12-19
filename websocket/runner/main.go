package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
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

var controllerConn *websocket.Conn // WebSocket connection to the controller

// Task execution routes
func setupRoutes() {
	http.HandleFunc("/example", func(w http.ResponseWriter, r *http.Request) {
		// Simulate task execution
		body := new(strings.Builder)
		_, _ = io.Copy(body, r.Body)
		response := fmt.Sprintf("Processed example task with body: %s", body.String())
		w.Write([]byte(response))
	})
}

// WebSocket connection to the controller
func connectToController() error {
	u := "ws://localhost:8080/connect"
	fmt.Printf("Connecting to controller at %s...\n", u)
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	controllerConn = conn
	fmt.Println("Connected to controller")
	return nil
}

// Listen for tasks and handle them
func listenForTasks() {
	for {
		var task Task
		err := controllerConn.ReadJSON(&task)
		if err != nil {
			fmt.Println("Error reading task from controller:", err)
			return
		}

		fmt.Printf("Received task %d: %s %s\n", task.ID, task.Method, task.URL)

		// Forward the task to the corresponding HTTP handler
		executeTaskViaHTTP(task)
	}
}

// Execute a task using a local HTTP handler and capture the response
func executeTaskViaHTTP(task Task) {
	req, err := http.NewRequest(task.Method, "http://localhost:9000"+task.URL, bytes.NewBuffer([]byte(task.Body)))
	if err != nil {
		fmt.Printf("Error creating HTTP request for task %d: %s\n", task.ID, err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error executing HTTP handler for task %d: %s\n", task.ID, err)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, _ := io.ReadAll(resp.Body)

	// Send the response back to the controller
	sendResponseToController(TaskResponse{
		TaskID:     task.ID,
		StatusCode: resp.StatusCode,
		Body:       string(body),
	})
}

// Send a response back to the controller
func sendResponseToController(response TaskResponse) {
	if controllerConn != nil {
		err := controllerConn.WriteJSON(response)
		if err != nil {
			fmt.Println("Error sending response to controller:", err)
		}
	}
}

func main() {
	// Start the HTTP server for task handlers
	go func() {
		setupRoutes()
		fmt.Println("Starting HTTP server on http://localhost:9000")
		http.ListenAndServe(":9000", nil)
	}()

	// Connect to the controller and listen for tasks
	err := connectToController()
	if err != nil {
		fmt.Println("Failed to connect to controller:", err)
		return
	}
	defer controllerConn.Close()

	listenForTasks()
}
