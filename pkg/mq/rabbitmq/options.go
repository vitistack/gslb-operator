package rabbitmq

import (
	"log/slog"
	"time"
)

type brokerOption[T any] func(broker *Broker[T])

func WithQueue[T any](queue string) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.queue = queue
	}
}

func WithDLX[T any](dlx string) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.dlx = dlx
	}
}

func WithDLQ[T any](dlq string) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.dlq = dlq
	}
}

func WithPrefetch[T any](prefetch int) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.prefetch = prefetch
	}
}

func WithRetryConnectionBackOff[T any](d time.Duration) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.retryConnectionBackoff = d
	}
}

func WithLogger[T any](l *slog.Logger) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.logger = l
	}
}
