package transport

import "context"

type Transport interface {
	GetJSON(ctx context.Context, path string, out any) error
	PostJSON(ctx context.Context, path string, body, out any) error
	PutJSON(ctx context.Context, path string, body, out any) error
	PatchJSON(ctx context.Context, path string, body, out any) error
	Delete(ctx context.Context, path string) error
}
