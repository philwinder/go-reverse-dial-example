package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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

type WorkerConnection struct {
	ID   int
	Conn *websocket.Conn
}

var (
	workers     = make([]WorkerConnection, 0)
	workersLock sync.Mutex
	taskID      int
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (customize for security)
	},
}

func connectWorkerHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading to WebSocket:", err)
		return
	}

	workerID := len(workers) + 1
	worker := WorkerConnection{ID: workerID, Conn: conn}

	workersLock.Lock()
	workers = append(workers, worker)
	workersLock.Unlock()

	fmt.Printf("Worker %d connected\n", workerID)

	// Listen for responses from the worker
	go func() {
		for {
			var response TaskResponse
			err := conn.ReadJSON(&response)
			if err != nil {
				fmt.Printf("Worker %d disconnected\n", workerID)
				removeWorker(workerID)
				return
			}
			fmt.Printf("Response for Task %d: %d - %s\n", response.TaskID, response.StatusCode, response.Body)
		}
	}()
}

func removeWorker(workerID int) {
	workersLock.Lock()
	defer workersLock.Unlock()
	for i, worker := range workers {
		if worker.ID == workerID {
			workers = append(workers[:i], workers[i+1:]...)
			break
		}
	}
}

func sendTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid task format", http.StatusBadRequest)
		return
	}

	workersLock.Lock()
	defer workersLock.Unlock()

	if len(workers) == 0 {
		http.Error(w, "No workers connected", http.StatusServiceUnavailable)
		return
	}

	// Assign a unique task ID and send the task to the first worker
	taskID++
	task.ID = taskID
	worker := workers[0]
	err := worker.Conn.WriteJSON(task)
	if err != nil {
		fmt.Printf("Error sending task to Worker %d: %s\n", worker.ID, err)
		removeWorker(worker.ID)
		http.Error(w, "Failed to send task to worker", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Task %d sent to Worker %d\n", task.ID, worker.ID)
}

func sendDummyTasks() {
	for {
		// Wait for 5 seconds between tasks
		time.Sleep(5 * time.Second)

		workersLock.Lock()
		if len(workers) == 0 {
			fmt.Println("No workers connected. Skipping dummy task.")
			workersLock.Unlock()
			continue
		}

		// Create a dummy task
		taskID++
		task := Task{
			ID:     taskID,
			Method: "POST",
			URL:    "/example",
			Body:   fmt.Sprintf("This is dummy task %d", taskID),
		}

		// Send the task to all connected workers
		for _, worker := range workers {
			err := worker.Conn.WriteJSON(task)
			if err != nil {
				fmt.Printf("Error sending dummy task to Worker %d: %s\n", worker.ID, err)
				removeWorker(worker.ID)
			} else {
				fmt.Printf("Dummy task %d sent to Worker %d\n", task.ID, worker.ID)
			}
		}
		workersLock.Unlock()
	}
}

func main() {
	http.HandleFunc("/connect", connectWorkerHandler) // Workers connect here
	http.HandleFunc("/send-task", sendTaskHandler)    // Send tasks to workers

	// Start the dummy task-sending routine
	go sendDummyTasks()

	fmt.Println("Controller running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
