package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"math/rand"
	"strconv"
)

func (n *Node) handleSingleCast(msg Message, needSend bool) {
	log.Printf("Handling single cast message: %+v", msg)
	messageKey := fmt.Sprintf("%s_%s", msg.SourceID, msg.MessageID)

	n.mu.Lock()
	if n.processed[messageKey] {
		n.mu.Unlock()
		return
	}
	n.processed[messageKey] = true
	n.mu.Unlock()

	n.updateValue(msg.Value, SingleCast)

	if needSend {
		// Отправляем сообщение соседним узлам через HTTP
		portStart := 8080
		for portNum := 0; portNum < n.NumOfNodes; portNum++ {
			go func(neighborURL string, port string) {
				if port == n.Port {
					return
				}
				newMsg := msg
				newMsg.SourceID = n.ID
				if err := n.sendHTTPMessage(neighborURL+port, newMsg); err != nil {
					log.Printf("Failed to send single cast to %s: %v", neighborURL, err)
				}
			}(fmt.Sprintf("http://node%d:", portNum+1), fmt.Sprint(portStart+portNum))
		}
	}

	messageCounter.WithLabelValues(n.ID, string(SingleCast)).Inc()
}

func (n *Node) handleMultiCast(msg Message) {
	messageKey := fmt.Sprintf("%s_%s", msg.SourceID, msg.MessageID)

	n.mu.Lock()
	if n.processed[messageKey] {
		n.mu.Unlock()
		return
	}
	n.processed[messageKey] = true
	n.mu.Unlock()

	n.updateValue(msg.Value, MultiCast)
	messageCounter.WithLabelValues(n.ID, string(MultiCast)).Inc()
}

func (n *Node) handleBroadCast(msg Message) {
	messageKey := fmt.Sprintf("%s_%s", msg.SourceID, msg.MessageID)

	n.mu.Lock()
	if n.processed[messageKey] {
		n.mu.Unlock()
		return
	}
	n.processed[messageKey] = true
	n.mu.Unlock()

	n.updateValue(msg.Value, BroadCast)

	// Если это супер-узел, отправляем сообщение в соседние группы
	if n.IsSuperNode {
		for groupId := 1; groupId <= n.NumOfGroups; groupId++ {
			groupStr := fmt.Sprint(groupId)
			if n.GroupID == groupStr {
				return
			}
			go func(groupStr string) {
				newmsg := msg
				newmsg.GroupID = groupStr
				writer := kafka.NewWriter(kafka.WriterConfig{
					Brokers:  []string{"kafka:29092"},
					Topic:    "broadcast-" + newmsg.GroupID,
					Balancer: &kafka.LeastBytes{},
				})
				defer writer.Close()

				jsonData, _ := json.Marshal(newmsg)
				if err := writer.WriteMessages(context.Background(), kafka.Message{
					Value: jsonData,
				}); err != nil {
					log.Printf("Failed to write broadcast message to Kafka: %v", err)
				}
			}(groupStr)
		}

	}

	messageCounter.WithLabelValues(n.ID, string(BroadCast)).Inc()
}

func (n *Node) handleGossip(msg Message) {
	messageKey := fmt.Sprintf("%s_%s", msg.SourceID, msg.MessageID)

	n.mu.Lock()
	if n.processed[messageKey] {
		n.mu.Unlock()
		return
	}
	n.processed[messageKey] = true
	n.mu.Unlock()

	n.updateValue(msg.Value, Gossip)

	// Отправляем сообщение случайному подмножеству соседей
	if len(n.Neighbors) > 0 {
		neighbors := make([]string, len(n.Neighbors))
		copy(neighbors, n.Neighbors)
		rand.Shuffle(len(neighbors), func(i, j int) {
			neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
		})

		// Отправляем примерно половине соседей (минимум 1)
		numToSend := len(neighbors) / 2
		if numToSend == 0 {
			numToSend = 1
		}

		for i := 0; i < numToSend && i < len(neighbors); i++ {
			go func(neighborURL string) {
				newMsg := msg
				newMsg.SourceID = n.ID
				if err := n.sendHTTPMessage(neighborURL, newMsg); err != nil {
					log.Printf("Failed to send gossip to %s: %v", neighborURL, err)
				}
			}(neighbors[i])
		}
	}

	maxNeighbors := 2 // слева и справа
	portStart := 8080
	portEnd := portStart + n.NumOfNodes - 1 // 8080+6-1 = 8085
	neighbors := make([]string, 0)
	myport, _ := strconv.Atoi(n.Port)
	idxNeighbor := -2
	for {
		if len(neighbors) >= maxNeighbors*2 {
			break
		}
		if idxNeighbor == 0 {
			idxNeighbor++
		}

		port := myport + idxNeighbor
		if port < portStart {
			port = portEnd - (portStart - port) // port = portStart-1 -> port = portEnd
		} else if port > portEnd {
			port = portStart + (port - portEnd)
		}
		nodeId := port - 8080 + 1 // node1:8080 -> 8080-8080+1 = 1
		neighbors = append(neighbors, fmt.Sprintf("http://node%d:%d", nodeId, port))
		idxNeighbor++
	}
	for _, neighbor := range neighbors {
		go func(neighborURL string) {
			newMsg := msg
			newMsg.SourceID = n.ID
			if err := n.sendHTTPMessage(neighborURL, newMsg); err != nil {
				log.Printf("Failed to send single cast to %s: %v", neighborURL, err)
			}
		}(neighbor)
	}

	messageCounter.WithLabelValues(n.ID, string(Gossip)).Inc()
}
