package events

// used to limit the amount of concurrent threads that perform an action
type Semaphore chan struct{}

func NewSemaphore(buffer int) Semaphore {
	return make(Semaphore, buffer)
}

// will block until the semaphore is aquired
func (s *Semaphore) Aquire() {
	*s <- struct{}{}
}

// release the semaphore to allow new ones to aquire it
func (s *Semaphore) Release() {
	<-*s
}
