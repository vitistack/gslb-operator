package pool

import (
	"errors"
	"testing"
	"time"
)

type MockJob struct {
	done chan struct{}
}

func (mj *MockJob) Execute() error {
	select {
	case mj.done <- struct{}{}: // signal complete
	default:
	}
	return nil
}

func (mj *MockJob) OnFailure(err error) {
	// no need to anything
}

func (mj *MockJob) OnSuccess() {}

func TestStart(t *testing.T) {
	wp := NewWorkerPool(2, 10)
	wp.Start()

	if got := wp.NumWorkers(); got != 2 {
		t.Fatalf("expected 2 workers after Start, but got: %d", got)
	}
	wp.Stop()
}

func TestStop(t *testing.T) {

}

func TestPutExecutesJob(t *testing.T) {
	wp := NewWorkerPool(1, 10)
	wp.Start()
	defer wp.Stop()

	j := &MockJob{done: make(chan struct{}, 1)}

	if err := wp.Put(j); err != nil {
		t.Fatalf("unexpected Put error: %v", err)
	}

	select {
	case <-j.done:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("job was not executed within timeout")
	}
}

func TestPutOnClosedPool(t *testing.T) {
	wp := NewWorkerPool(2, 10)
	wp.Start()
	wp.Stop()

	err := wp.Put(&MockJob{done: make(chan struct{})})
	if !errors.Is(err, ErrPutOnClosedPool) {
		t.Fatalf("expected error %v, after attempted put on stopped pool, but got: %v", ErrPutOnClosedPool, err)
	}
}

func TestScaleTo(t *testing.T) {
	wp := NewWorkerPool(4, 5)
	wp.Start()

	wp.ScaleTo(10)

	time.Sleep(time.Millisecond * 50) // let workers spin up

	if got := wp.NumWorkers(); got != 10 {
		t.Fatalf("expected: 10 workers after ScaleTo(), but got: %v", got)
	}

	wp.Stop()
}
