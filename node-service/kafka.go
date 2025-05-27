package main

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

func waitForKafka() {
	log.Println("Waiting for Kafka to be ready...")
	for i := 0; i < 60; i++ { // Ждем до 60 секунд
		conn, err := kafka.Dial("tcp", "kafka:29092")
		if err == nil {
			conn.Close()
			log.Println("Kafka is ready!")
			return
		}
		log.Printf("Kafka not ready, retrying... (%d/60)", i+1)
		time.Sleep(1 * time.Second)
	}
	log.Fatal("Kafka is not available after 60 seconds")
}

func (n *Node) startKafkaConsumer(ctx context.Context) {
	// Multicast consumer
	go func() {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{"kafka:29092"},
			Topic:   "multicast",
			GroupID: n.ID,
		})
		defer reader.Close()

		log.Printf("Starting multicast consumer for node %s", n.ID)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("Error reading multicast message: %v", err)
					time.Sleep(1 * time.Second)
					continue
				}

				var message Message
				if err := json.Unmarshal(msg.Value, &message); err != nil {
					log.Printf("Error unmarshaling multicast message: %v", err)
					continue
				}

				n.handleMultiCast(message)
			}
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

		log.Printf("Starting broadcast consumer for node %s", n.ID)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("Error reading broadcast message: %v", err)
					time.Sleep(1 * time.Second)
					continue
				}

				var message Message
				if err := json.Unmarshal(msg.Value, &message); err != nil {
					log.Printf("Error unmarshaling broadcast message: %v", err)
					continue
				}

				n.handleBroadCast(message)
			}
		}
	}()
}
