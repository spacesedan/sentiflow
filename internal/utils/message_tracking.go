package utils

import (
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var messageMap sync.Map

func TrackMessage(contentID string, msg *kafka.Message) {
	messageMap.Store(contentID, msg)
}

func GetMessageForContent(contentID string) (*kafka.Message, bool) {
	msg, ok := messageMap.Load(contentID)
	if !ok {
		return nil, false
	}
	messageMap.Delete(contentID)
	return msg.(*kafka.Message), true
}
