package utils

import (
	"log/slog"
	"sync"
	"time"
)

const (
	BATCH_SIZE    = 10
	BATCH_TIMEOUT = time.Second * 5
)

type BatchBuffer[T any] struct {
	buffer     []T
	bufferLock sync.Mutex
}

func NewBatchBuffer[T any]() *BatchBuffer[T] {
	return &BatchBuffer[T]{
		buffer: make([]T, 0, BATCH_SIZE),
	}
}

func (b *BatchBuffer[T]) Add(item T) {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	b.buffer = append(b.buffer, item)
}

func (b *BatchBuffer[T]) GetAndClear() []T {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	if len(b.buffer) == 0 {
		return nil
	}

	batch := b.buffer
	b.buffer = make([]T, 0, BATCH_SIZE)
	return batch
}

func (b *BatchBuffer[T]) Size() int {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()
	return len(b.buffer)
}

func (b *BatchBuffer[T]) HasData() bool {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()
	return len(b.buffer) > 0
}

func (b *BatchBuffer[T]) Peek() []T {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	return append([]T(nil), b.buffer...)
}

func (b *BatchBuffer[T]) Flush() {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	b.buffer = make([]T, 0, BATCH_SIZE)
}

func (b *BatchBuffer[T]) LogBatchProcessing(batchType string) {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	slog.Info("[BatchBuffer] Processing batch",
		slog.String("type", batchType),
		slog.Int("batch_size", len(b.buffer)))
}
