package update

import "context"

type Updater interface {
	Create(...Record) error
	Delete(string, ...string) error
	Synchronize(context.Context)
}
