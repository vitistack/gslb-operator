package transport

type Transport interface {
	Execute(string) (string, error)
	Close() error
}
