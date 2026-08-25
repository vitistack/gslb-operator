package rabbitmq

import (
	"log/slog"
)

type brokerOption[T any] func(broker *Broker[T])

func WithExchange[T any](exchange string) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.exchange = exchange
	}
}

func WithQueue[T any](queue string) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.queue = queue
	}
}

func WithFanout[T any]() brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.fanout = true
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

func WithRetryer[T any](retryer Retryer) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.retry = retryer
	}
}

//func WithRetryConnectionBackOff[T any](d time.Duration) brokerOption[T] {
//	return func(broker *Broker[T]) {
//		broker.retryConnectionBackoff = d
//	}
//}

func WithLogger[T any](l *slog.Logger) brokerOption[T] {
	return func(broker *Broker[T]) {
		broker.logger = l
	}
}
