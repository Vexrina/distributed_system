package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	nodeValueUpdates = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_value_updates",
			Help: "Node status (0=off,1=singlecast,2=multicast,3=broadcast,4=gossip)",
		},
		[]string{"node_id"},
	)
	messageCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "message_count",
			Help: "Count of processed messages by type",
		},
		[]string{"node_id", "message_type"},
	)
	propagationTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "message_propagation_time",
			Help:    "Time taken for message propagation in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
		},
		[]string{"message_type", "source_node"},
	)
	healthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_health",
			Help: "Node health status (1=healthy, 0=unhealthy)",
		},
		[]string{"node_id"},
	)
)

func init() {
	log.Println("Registering Prometheus metrics...")
	prometheus.MustRegister(nodeValueUpdates)
	prometheus.MustRegister(messageCounter)
	prometheus.MustRegister(propagationTime)
	prometheus.MustRegister(healthStatus)
	log.Println("Prometheus metrics registered successfully")
}

func (n *Node) resetMetrics() {
	// Сбрасываем метрику для данного узла (удаляем все временные ряды с этой меткой node_id)
	nodeValueUpdates.DeleteLabelValues(n.ID)
}

func (n *Node) updateValue(newValue string, msgType MessageType) {
	log.Printf("Updating value to %s with type %s", newValue, msgType)
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Value = newValue

	// Сбрасываем старые метрики для этого узла
	n.resetMetrics()

	// Обновляем метрики
	status := 1.0
	switch msgType {
	case SingleCast:
		status = 1.0
	case MultiCast:
		status = 2.0
	case BroadCast:
		status = 3.0
	case Gossip:
		status = 4.0
	}
	log.Printf("Setting node_value_updates metric for node %s to %f", n.ID, status)
	nodeValueUpdates.WithLabelValues(n.ID).Set(status)
	log.Printf("Node %s updated value to %s via %s", n.ID, newValue, msgType)
}

func (n *Node) getValue() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Value
}

func parseNeighbors(neighborsStr string) []string {
	if neighborsStr == "" {
		return []string{}
	}
	return strings.Split(neighborsStr, ",")
}

func createKafkaTopics(topics []string) error {
	// Создаем конфигурацию для админа Kafka
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0

	// Создаем админа Kafka
	admin, err := sarama.NewClusterAdmin([]string{"kafka:29092"}, config)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin: %v", err)
	}
	defer admin.Close()

	// Создаем топики
	for _, topic := range topics {
		err := admin.CreateTopic(topic, &sarama.TopicDetail{
			NumPartitions:     3,
			ReplicationFactor: 1,
		}, false)
		if err != nil {
			if err.Error() == "TopicAlreadyExistsError" {
				log.Printf("Topic %s already exists, skipping creation", topic)
				continue
			}
			return fmt.Errorf("failed to create topic %s: %v", topic, err)
		}
		log.Printf("Created Kafka topic: %s", topic)
	}

	return nil
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

	numOfGroupsStr := os.Getenv("NUM_OF_GROUPS")
	numofGroups, _ := strconv.Atoi(numOfGroupsStr)
	isSuperNode := os.Getenv("IS_SUPER_NODE") == "true"

	// Получаем список соседей из переменной окружения
	neighborsStr := os.Getenv("NEIGHBORS")
	neighbors := parseNeighbors(neighborsStr)

	numOfNodesStr := os.Getenv("NUM_OF_NODES")
	numOfNodes, err := strconv.Atoi(numOfNodesStr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting node %s on port %s, group %s, super_node: %v", nodeID, port, groupID, isSuperNode)
	log.Printf("Neighbors: %v", neighbors)

	// Ждем готовности Kafka
	waitForKafka()

	// Создаем список топиков для всех групп
	var kafkaTopics []string
	kafkaTopics = append(kafkaTopics, "multicast") // добавляем топик для multicast
	for i := 1; i <= numofGroups; i++ {
		kafkaTopics = append(kafkaTopics, fmt.Sprintf("broadcast-group-%d", i))
	}

	// Создаем топики в Kafka
	if err := createKafkaTopics(kafkaTopics); err != nil {
		log.Printf("Warning: failed to create Kafka topics: %v", err)
	}

	// Формируем список топиков для текущего узла
	nodeKafkaTopics := []string{"multicast", fmt.Sprintf("broadcast-group-%s", strings.TrimPrefix(groupID, "group-"))}

	node := NewNode(
		nodeID,
		numOfNodes,
		neighbors,
		nodeKafkaTopics,
		groupID,
		isSuperNode,
		port,
		numofGroups,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Kafka consumer
	go node.startKafkaConsumer(ctx)

	// Start HTTP server in goroutine
	go node.startHTTPServer(port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Node %s is running. Press Ctrl+C to stop.", nodeID)
	<-sigChan

	log.Printf("Shutting down node %s...", nodeID)
	cancel()
}
