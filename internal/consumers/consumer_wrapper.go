package consumers

import (
	"context"
	"sync/atomic"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type ConsumerWrapper struct {
	fn     func(ctx context.Context, consumer *kafka.Consumer, health ...*atomic.Bool)
	health []*atomic.Bool
}

func WrapConsumer(fn func(ctx context.Context, consumer *kafka.Consumer, health ...*atomic.Bool), health ...*atomic.Bool) ConsumerWrapper {
	return ConsumerWrapper{
		fn:     fn,
		health: health,
	}
}

func (cw ConsumerWrapper) WithHealthCheck(health *atomic.Bool) ConsumerWrapper {
	cw.health = append(cw.health, health)
	return cw
}

func (cw ConsumerWrapper) Handler() func(ctx context.Context, consumer *kafka.Consumer) {
	return func(ctx context.Context, consumer *kafka.Consumer) {
		cw.fn(ctx, consumer, cw.health...)
	}
}
