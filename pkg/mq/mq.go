package mq

import (
	"context"
)

type MessageHandler[T any] func(context.Context, T) error

type MessageBroker[T any] interface {
	Publish(context.Context, T) error
	Subscribe(context.Context, MessageHandler[T]) error
}
