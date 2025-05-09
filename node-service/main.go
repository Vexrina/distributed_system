package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

type MessageType string

const (
	SingleCast MessageType = "single_cast"
	MultiCast  MessageType = "multicast"
	BroadCast  MessageType = "broadcast"
	Gossip     MessageType = "gossip"
)

type Message struct {
	Type      MessageType `json:"type"`
	Value     string      `json:"value"`
	SourceID  string      `json:"source_id"`
	GroupID   string      `json:"group_id,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

type Node struct {
	ID          string   `json:"id"`
	Value       string   `json:"value"`
	Neighbors   []string `json:"neighbors"`
	KafkaTopics []string `json:"kafka_topics"`
	GroupID     string   `json:"group_id"`
	IsSuperNode bool     `json:"is_super_node"`
	mu          sync.RWMutex
	processed   map[string]bool
}

type UpdateRequest struct {
	Value   string      `json:"value"`
	Type    MessageType `json:"type"`
	GroupID string      `json:"group_id,omitempty"`
}

var (
	nodeInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_value_updates",
			Help: "Number of value updates for each node",
		},
		[]string{"node_id", "message_type"},
	)
)

func init() {
	prometheus.MustRegister(nodeInfo)
}

func NewNode(id string, neighbors []string, kafkaTopics []string, groupID string, isSuperNode bool) *Node {
	return &Node{
		ID:          id,
		Value:       "initial",
		Neighbors:   neighbors,
		KafkaTopics: kafkaTopics,
		GroupID:     groupID,
		IsSuperNode: isSuperNode,
		processed:   make(map[string]bool),
	}
}

func (n *Node) updateValue(newValue string, msgType MessageType) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Value = newValue
	nodeInfo.WithLabelValues(n.ID, string(msgType)).Inc()
}

func (n *Node) getValue() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Value
}

func (n *Node) handleSingleCast(msg Message) {
	if n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] {
		return
	}

	n.updateValue(msg.Value, SingleCast)
	n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] = true

	// Отправляем сообщение соседним нодам через HTTP
	for _, neighbor := range n.Neighbors {
		go func(neighborURL string) {
			msg.SourceID = n.ID
			jsonData, _ := json.Marshal(msg)
			http.Post(neighborURL+"/message", "application/json", bytes.NewBuffer(jsonData))
		}(neighbor)
	}
}

func (n *Node) handleMultiCast(msg Message) {
	if n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] {
		return
	}

	n.updateValue(msg.Value, MultiCast)
	n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] = true
}

func (n *Node) handleBroadCast(msg Message) {
	if n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] {
		return
	}

	n.updateValue(msg.Value, BroadCast)
	n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] = true

	// Если это супер-нода, отправляем сообщение в соседние группы
	if n.IsSuperNode {
		writer := kafka.NewWriter(kafka.WriterConfig{
			Brokers: []string{"kafka:29092"},
			Topic:   "broadcast-" + msg.GroupID,
		})
		defer writer.Close()

		jsonData, _ := json.Marshal(msg)
		writer.WriteMessages(context.Background(), kafka.Message{
			Value: jsonData,
		})
	}
}

func (n *Node) handleGossip(msg Message) {
	if n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] {
		return
	}

	n.updateValue(msg.Value, Gossip)
	n.processed[fmt.Sprintf("%s_%d", msg.SourceID, msg.Timestamp)] = true

	// Отправляем сообщение случайному подмножеству соседей
	neighbors := make([]string, len(n.Neighbors))
	copy(neighbors, n.Neighbors)
	rand.Shuffle(len(neighbors), func(i, j int) {
		neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
	})

	// Отправляем примерно половине соседей
	for i := 0; i < len(neighbors)/2; i++ {
		go func(neighborURL string) {
			msg.SourceID = n.ID
			jsonData, _ := json.Marshal(msg)
			http.Post(neighborURL+"/message", "application/json", bytes.NewBuffer(jsonData))
		}(neighbors[i])
	}
}

func (n *Node) startHTTPServer(port string) {
	r := mux.NewRouter()

	r.HandleFunc("/value", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]string{"value": n.getValue()})
			return
		}

		if r.Method == http.MethodPost {
			var req UpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			msg := Message{
				Type:      req.Type,
				Value:     req.Value,
				SourceID:  n.ID,
				GroupID:   req.GroupID,
				Timestamp: time.Now().UnixNano(),
			}

			switch req.Type {
			case SingleCast:
				n.handleSingleCast(msg)
			case MultiCast:
				// Отправляем в Kafka топик для multicast
				writer := kafka.NewWriter(kafka.WriterConfig{
					Brokers: []string{"kafka:29092"},
					Topic:   "multicast",
				})
				defer writer.Close()

				jsonData, _ := json.Marshal(msg)
				writer.WriteMessages(context.Background(), kafka.Message{
					Value: jsonData,
				})
			case BroadCast:
				// Отправляем в Kafka топик для broadcast
				writer := kafka.NewWriter(kafka.WriterConfig{
					Brokers: []string{"kafka:29092"},
					Topic:   "broadcast-" + n.GroupID,
				})
				defer writer.Close()

				jsonData, _ := json.Marshal(msg)
				writer.WriteMessages(context.Background(), kafka.Message{
					Value: jsonData,
				})
			case Gossip:
				n.handleGossip(msg)
			}

			w.WriteHeader(http.StatusOK)
		}
	})

	r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var msg Message
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			switch msg.Type {
			case SingleCast:
				n.handleSingleCast(msg)
			case Gossip:
				n.handleGossip(msg)
			}

			w.WriteHeader(http.StatusOK)
		}
	})

	r.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting HTTP server on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func (n *Node) startKafkaConsumer() {
	// Multicast consumer
	go func() {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{"kafka:29092"},
			Topic:   "multicast",
			GroupID: n.ID,
		})
		defer reader.Close()

		for {
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Error reading message: %v", err)
				continue
			}

			var message Message
			if err := json.Unmarshal(msg.Value, &message); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			n.handleMultiCast(message)
		}
	}()

	// Broadcast consumer
	go func() {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{"kafka:29092"},
			Topic:   "broadcast-" + n.GroupID,
			GroupID: n.ID,
		})
		defer reader.Close()

		for {
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Error reading message: %v", err)
				continue
			}

			var message Message
			if err := json.Unmarshal(msg.Value, &message); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			n.handleBroadCast(message)
		}
	}()
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "node-1"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	groupID := os.Getenv("GROUP_ID")
	if groupID == "" {
		groupID = "group-1"
	}

	isSuperNode := os.Getenv("IS_SUPER_NODE") == "true"

	neighbors := []string{"http://node2:8081", "http://node3:8082"}
	kafkaTopics := []string{"multicast", "broadcast-" + groupID}

	node := NewNode(nodeID, neighbors, kafkaTopics, groupID, isSuperNode)

	// Start Kafka consumer
	go node.startKafkaConsumer()

	// Start HTTP server
	go node.startHTTPServer(port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
