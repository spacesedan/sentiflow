package kafka_client

import "os"

type KafkaConfig struct {
	Broker  string
	GroupID string
	Topic   string
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func GetKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Broker:  getEnv("KAFKA_BROKER", "localhost:29092"),
		GroupID: getEnv("KAFKA_CONSUMER_GROUP_ID", "sentiflow-consumer-group"),
		Topic:   getEnv("KAFKA_CONSUMER_TOPIC", KAFKA_TOPIC_SENTIMENT_BATCHES),
	}
}
