package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats-server/v2/server"
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

var taskID int

func startEmbeddedNATSServer() *server.Server {
	opts := &server.Options{
		Port:   4222,
		NoLog:  true,
		NoSigs: true,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		log.Fatalf("Error starting NATS server: %v", err)
	}

	// Start the server in a goroutine
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		log.Fatalf("NATS server failed to start")
	}
	log.Println("Embedded NATS server started")
	return s
}

func main() {
	// Start the embedded NATS server
	natsServer := startEmbeddedNATSServer()
	defer natsServer.Shutdown()
	log.Printf("Embedded NATS server started at %s", natsServer.ClientURL())

	// Connect to the embedded NATS server
	nc, err := nats.Connect(fmt.Sprintf("nats://%s", natsServer.Addr().String()))
	if err != nil {
		log.Fatalf("Error connecting to NATS server: %v", err)
	}
	defer nc.Close()

	// Subscribe to the "responses" subject to receive task responses
	_, err = nc.Subscribe("responses", func(msg *nats.Msg) {
		var response TaskResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			log.Printf("Error unmarshalling response: %v", err)
			return
		}
		log.Printf("Received response for Task %d: %d - %s", response.TaskID, response.StatusCode, response.Body)
	})
	if err != nil {
		log.Fatalf("Error subscribing to responses: %v", err)
	}

	// Periodically send dummy tasks
	go func() {
		for {
			time.Sleep(5 * time.Second)

			taskID++
			task := Task{
				ID:     taskID,
				Method: "POST",
				URL:    "/example",
				Body:   fmt.Sprintf("Dummy task %d body", taskID),
			}

			data, err := json.Marshal(task)
			if err != nil {
				log.Printf("Error marshalling task: %v", err)
				continue
			}

			// Publish the task to the "tasks" subject
			if err := nc.Publish("tasks", data); err != nil {
				log.Printf("Error publishing task: %v", err)
			} else {
				log.Printf("Published Task %d to runners", taskID)
			}
		}
	}()

	// Keep the controller running
	select {}
}
