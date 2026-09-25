package transport

type Transport interface {
	Execute(string) (string, error)
	Close() error
}

type Retryer interface {
	Retry(func() error) error
}

type RetryFunc func(func() error) error

func (rf RetryFunc) Retry(fn func() error) error {
	return rf(fn)
}
