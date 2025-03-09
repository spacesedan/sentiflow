package kafka_client

import "os"

const KAFKA_TOPIC = "user-content"

type KafkaConfig struct {
	Broker  string
	GroupID string
}

func GetKafkaConfig() KafkaConfig {
	var broker string
	var groupID string

	if os.Getenv("KAFKA_BROKER") != "" {
		broker = os.Getenv("KAFKA_BROKER")
	} else {
		broker = "localhost:29092"
	}

	if os.Getenv("KAFKA_CONSUMER_GROUP_ID") != "" {
		groupID = os.Getenv("KAFKA_CONSUMER_GROUP_ID")
	} else {
		groupID = "sentiflow-consumer-group"
	}

	return KafkaConfig{
		Broker:  broker,
		GroupID: groupID,
	}
}
