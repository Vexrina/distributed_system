package main

import (
	"sync"
	"time"
)

type MessageType string

const (
	SingleCast MessageType = "single_cast"
	MultiCast  MessageType = "multicast"
	BroadCast  MessageType = "broadcast"
	Gossip     MessageType = "gossip"
)

type Message struct {
	Type                 MessageType `json:"type"`
	Value                string      `json:"value"`
	SourceID             string      `json:"source_id"`
	GroupID              string      `json:"group_id,omitempty"`
	Timestamp            int64       `json:"timestamp"`
	MessageID            string      `json:"message_id"`
	FromAnotherSuperNode bool        `json:"from_another_super_node"`
	Visited              []string    `json:"visited,omitempty"`
}

type Node struct {
	ID            string   `json:"id"`
	Value         Value    `json:"value"`
	Neighbors     []string `json:"neighbors"`
	KafkaTopics   []string `json:"kafka_topics"`
	GroupID       string   `json:"group_id"`
	IsSuperNode   bool     `json:"is_super_node"`
	NumOfGroups   int      `json:"num_of_groups"`
	Port          string   `json:"port"`
	NumOfNodes    int      `json:"num_of_nodes"`
	mu            sync.RWMutex
	processed     map[string]bool
	PercentOfLoss int
}

type Value struct {
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func NewValue(value string) Value {
	return Value{
		Value:     value,
		Timestamp: time.Now(),
	}
}

type UpdateRequest struct {
	Value   string      `json:"value"`
	Type    MessageType `json:"type"`
	GroupID string      `json:"group_id,omitempty"`
}

func NewNode(
	id string, numOfNodes int,
	neighbors []string, kafkaTopics []string,
	groupID string, isSuperNode bool, port string,
	numofGroups int,
	percentOfLoss int,
) *Node {
	return &Node{
		ID:            id,
		Value:         NewValue("init"),
		Neighbors:     neighbors,
		KafkaTopics:   kafkaTopics,
		GroupID:       groupID,
		IsSuperNode:   isSuperNode,
		Port:          port,
		NumOfNodes:    numOfNodes,
		NumOfGroups:   numofGroups,
		processed:     make(map[string]bool),
		PercentOfLoss: percentOfLoss,
	}
}
