package pool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type Job interface {
	Execute() error
	OnFailure(error)
	OnSuccess()
}

type WorkerPool struct {
	numRunningWorkers uint // cant be negative
	minRunningWorkers uint
	jobs              BufferedJobQueue
	start             sync.Once
	stop              sync.Once
	quit              chan struct{}
	poolWg            *sync.WaitGroup
	lock              sync.Mutex
	closed            *atomic.Bool
}

func NewWorkerPool(minRunningWorkers, nonBlockingBufferSize uint) *WorkerPool {
	closed := &atomic.Bool{}
	closed.Store(true)

	return &WorkerPool{
		numRunningWorkers: 0,
		minRunningWorkers: minRunningWorkers,
		jobs:              make(chan Job, nonBlockingBufferSize),
		start:             sync.Once{},
		stop:              sync.Once{},
		quit:              make(chan struct{}),
		poolWg:            &sync.WaitGroup{},
		lock:              sync.Mutex{},
		closed:            closed,
	}
}

func (wp *WorkerPool) Start() {
	wg := sync.WaitGroup{}
	wp.closed.Store(false)
	wp.start.Do(func() {
		for range wp.minRunningWorkers {
			wg.Go(func() {
				wp.newWorker()
			})
		}
		wg.Wait() // blocks until all workers are spun up
	})
}

func (wp *WorkerPool) Stop() {
	wp.stop.Do(func() {
		wp.closed.Store(true)
		close(wp.quit)
		wp.poolWg.Wait()
		close(wp.jobs)
	})
}

func (wp *WorkerPool) Put(job Job) error {
	if wp.closed.Load() { // pool is stopped
		return ErrPutOnClosedPool
	}

	wp.scale()
	wp.jobs <- job

	return nil
}

func (wp *WorkerPool) scale() {
	if wp.jobs.Blocked() {
		wp.newWorker()
	}
}

// ScaleTo does not enforce running worker count, it just ensure that there are ATLEAST a minimum amount of workers running.
// this means that workers will only be spawned if there are fewer running workers than the target.
// otherwise, the life of a worker is completely handled inside the worker loop, and the only way to explicitly stop workers, is to call Stop()
func (wp *WorkerPool) ScaleTo(targetWorkers uint) {
	wp.lock.Lock()
	wp.minRunningWorkers = targetWorkers
	if wp.numRunningWorkers >= targetWorkers {
		wp.lock.Unlock()
		return // already reached worker count
	}
	newWorkers := targetWorkers - wp.numRunningWorkers // amount of workers to add
	wp.lock.Unlock()

	for range newWorkers {
		wp.newWorker()
	}
}

func (wp *WorkerPool) newWorker() {
	wp.lock.Lock()
	defer wp.lock.Unlock()

	wp.numRunningWorkers++
	id := uuid.New().ID()

	wp.poolWg.Add(1)
	go wp.worker(id)
}

func (wp *WorkerPool) worker(id uint32) {
	defer wp.poolWg.Done()

	for {
		select {
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}

			err := job.Execute()
			if err != nil {
				job.OnFailure(err)
			}else {
				job.OnSuccess()
			}

		case <-wp.quit:

			wp.lock.Lock()
			wp.numRunningWorkers--
			wp.lock.Unlock()

			return
		case <-time.After(IDLESTOP): // worker is idle
			wp.lock.Lock()
			if wp.numRunningWorkers > wp.minRunningWorkers {
				wp.numRunningWorkers--
				wp.lock.Unlock()
				return
			}
			wp.lock.Unlock()
		}
	}
}

func (wp *WorkerPool) NumWorkers() uint {
	wp.lock.Lock()
	defer wp.lock.Unlock()
	return wp.numRunningWorkers
}
