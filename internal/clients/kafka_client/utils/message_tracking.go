package utils

import (
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var messageMap sync.Map

func TrackMessage(postID string, msg *kafka.Message) {
	messageMap.Store(postID, msg)
}

func GetMessageForPost(postID string) (*kafka.Message, bool) {
	msg, ok := messageMap.Load(postID)
	if !ok {
		return nil, false
	}
	messageMap.Delete(postID)
	return msg.(*kafka.Message), true
}
