package main

import "sync"

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
	MessageID string      `json:"message_id"`
}

type Node struct {
	ID          string   `json:"id"`
	Value       string   `json:"value"`
	Neighbors   []string `json:"neighbors"`
	KafkaTopics []string `json:"kafka_topics"`
	GroupID     string   `json:"group_id"`
	IsSuperNode bool     `json:"is_super_node"`
	NumOfGroups int      `json:"num_of_groups"`
	Port        string   `json:"port"`
	NumOfNodes  int      `json:"num_of_nodes"`
	mu          sync.RWMutex
	processed   map[string]bool
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
) *Node {
	return &Node{
		ID:          id,
		Value:       "initial",
		Neighbors:   neighbors,
		KafkaTopics: kafkaTopics,
		GroupID:     groupID,
		IsSuperNode: isSuperNode,
		Port:        port,
		NumOfNodes:  numOfNodes,
		NumOfGroups: numofGroups,
		processed:   make(map[string]bool),
	}
}
