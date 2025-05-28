package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

func (n *Node) sendHTTPMessage(url string, msg Message) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := client.Post(url+"/message", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %d", resp.StatusCode)
	}

	return nil
}

func (n *Node) startHTTPServer(port string) {
	r := mux.NewRouter()

	// Добавляем health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Health check request received for node %s", n.ID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Добавляем metrics endpoint
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	r.HandleFunc("/value", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			log.Printf("Received POST request to /value for node %s", n.ID)
			var req UpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				log.Printf("Error decoding request: %v", err)
				http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
				return
			}
			log.Printf("Decoded request: %+v", req)

			messageID := fmt.Sprintf("%s_%d", n.ID, time.Now().UnixNano())
			msg := Message{
				Type:      req.Type,
				Value:     req.Value,
				SourceID:  n.ID,
				GroupID:   n.GroupID,
				Timestamp: time.Now().UnixNano(),
				MessageID: messageID,
			}
			log.Printf("Created message: %+v", msg)

			startTime := time.Now()

			switch req.Type {
			case SingleCast:
				log.Printf("Handling single cast message for node %s", n.ID)
				n.handleSingleCast(msg, true)
			case MultiCast:
				log.Printf("Handling multicast message for node %s", n.ID)
				// Отправляем в Kafka топик для multicast
				go func() {
					writer := kafka.NewWriter(kafka.WriterConfig{
						Brokers:  []string{"kafka:29092"},
						Topic:    "multicast",
						Balancer: &kafka.LeastBytes{},
					})
					defer writer.Close()

					jsonData, _ := json.Marshal(msg)
					if err := writer.WriteMessages(context.Background(), kafka.Message{
						Value: jsonData,
					}); err != nil {
						log.Printf("Failed to write multicast message to Kafka: %v", err)
					}
				}()
			case BroadCast:
				log.Printf("Handling broadcast message for node %s", n.ID)
				n.handleBroadCast(msg)
			case Gossip:
				log.Printf("Handling gossip message for node %s", n.ID)
				n.handleGossip(msg)
			}

			// Записываем метрику времени распространения
			propagationTime.WithLabelValues(string(req.Type), n.ID).Observe(time.Since(startTime).Seconds())

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "ok",
				"message_id": messageID,
			})
		}
	})

	r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var msg Message
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
				return
			}
			n.Value = msg.Value
			switch msg.Type {
			case SingleCast:
				n.handleSingleCast(msg, false)
			case Gossip:
				n.handleGossip(msg)
			default:
				log.Printf("Received unsupported message type via HTTP: %s", msg.Type)
			}

			w.WriteHeader(http.StatusOK)
		}
	})

	log.Printf("Starting HTTP server on port %s for node %s", port, n.ID)
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
