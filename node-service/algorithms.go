package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Новая функция для обработки сообщений с отслеживанием времени
func (n *Node) processMessageWithTiming(msg Message, msgType MessageType) {
	messageKey := fmt.Sprintf("%s_%s", msg.SourceID, msg.MessageID)

	// Проверяем потерю сообщения
	if rand.Intn(100) < n.PercentOfLoss {
		log.Printf("Message %s lost", messageKey)
		return
	}

	// Проверяем, не обрабатывали ли мы уже это сообщение
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.processed[messageKey] {
		return
	}
	n.processed[messageKey] = true
}

func (n *Node) handleSingleCast(msg Message, needSend bool) {
	log.Printf("Handling single cast message: %+v", msg)

	// Обрабатываем сообщение с отслеживанием времени
	n.processMessageWithTiming(msg, SingleCast)

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
}

func (n *Node) handleMultiCast(msg Message) {
	// Обрабатываем сообщение с отслеживанием времени
	n.processMessageWithTiming(msg, MultiCast)
}

func (n *Node) handleBroadCast(msg Message, needSend bool) {

	// Обрабатываем сообщение с отслеживанием времени
	n.processMessageWithTiming(msg, BroadCast)

	// Если это супер-узел, отправляем сообщение в соседние группы
	if n.IsSuperNode || !msg.FromAnotherSuperNode {
		for groupId := 1; groupId <= n.NumOfGroups; groupId++ {
			groupStr := fmt.Sprint(groupId)
			go func(groupStr string) {
				newmsg := msg
				newmsg.GroupID = groupStr
				newmsg.FromAnotherSuperNode = true
				writer := kafka.NewWriter(kafka.WriterConfig{
					Brokers:  []string{"kafka:29092"},
					Topic:    fmt.Sprintf("broadcast-group-%s", groupStr),
					Balancer: &kafka.LeastBytes{},
				})

				defer writer.Close()
				// log.Printf("Sending broadcast message to Kafka with groupID: %s", groupStr)
				jsonData, _ := json.Marshal(newmsg)
				if err := writer.WriteMessages(context.Background(), kafka.Message{
					Value: jsonData,
				}); err != nil {
					log.Printf("Failed to write broadcast message to Kafka topic: %s, error: %v", fmt.Sprintf("broadcast-group-%s", groupStr), err)
				}
			}(groupStr)
		}
	} else if needSend {
		go func(groupStr string) {
			newmsg := msg
			newmsg.GroupID = groupStr
			newmsg.FromAnotherSuperNode = false
			// Извлекаем номер группы из GroupID (например, из "group-1" получаем "1")
			groupNumber := strings.TrimPrefix(n.GroupID, "group-")
			writer := kafka.NewWriter(kafka.WriterConfig{
				Brokers:  []string{"kafka:29092"},
				Topic:    fmt.Sprintf("broadcast-group-%s", groupNumber),
				Balancer: &kafka.LeastBytes{},
			})
			defer writer.Close()
			// log.Printf("Sending broadcast message to Kafka with groupID: %s", groupStr)
			jsonData, _ := json.Marshal(newmsg)
			if err := writer.WriteMessages(context.Background(), kafka.Message{
				Value: jsonData,
			}); err != nil {
				log.Printf("Failed to write broadcast message to Kafka topic: %s, error: %v", fmt.Sprintf("broadcast-group-%s", groupNumber), err)
			}
		}(n.GroupID)
	}
}

func (n *Node) handleGossip(msg Message) {
	// Проверяем, получала ли уже эта нода сообщение
	for _, id := range msg.Visited {
		if id == n.ID {
			return // Уже получала, не обрабатываем и не пересылаем
		}
	}
	// Добавляем свой ID в список Visited
	msg.Visited = append(msg.Visited, n.ID)
	// Проверяем потерю сообщения
	if rand.Intn(100) < n.PercentOfLoss {
		log.Printf("Message %s lost", msg.Value)
		return
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
			// Не отправляем, если этот сосед уже получал сообщение
			for _, id := range newMsg.Visited {
				if strings.HasSuffix(neighborURL, newMsg.SourceID) || id == extractNodeIDFromURL(neighborURL) {
					return
				}
			}
			newMsg.SourceID = n.ID
			if err := n.sendHTTPMessage(neighborURL, newMsg); err != nil {
				log.Printf("Failed to send single cast to %s: %v", neighborURL, err)
			}
		}(neighbor)
	}
}

// Вспомогательная функция для извлечения ID ноды из URL
func extractNodeIDFromURL(url string) string {
	parts := strings.Split(url, ":")
	if len(parts) > 1 {
		return strings.TrimPrefix(parts[1], "//node")
	}
	return ""
}
