package utils

import (
	"encoding/json"
	"log/slog"
)

func SerializeToJSON(value interface{}) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		slog.Warn("[KafkaUtils] Failed to serialize JSON",
			slog.String("error", err.Error()))
		return nil, err
	}
	return data, nil
}

func DeserializeFromJSON(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		slog.Warn("[KafkaUtils] Failed to deserialize JSON",
			slog.String("error", err.Error()))
		return err
	}
	return err
}

func HandleConsumerError(err error) {
	if err == nil {
		return
	}
	slog.Error("[KafkaUtils] Kafka Consumer Error",
		slog.String("error", err.Error()))
}
