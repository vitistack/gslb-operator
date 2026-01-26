package lua

import (
	"runtime"
	"sync"

	glua "github.com/yuin/gopher-lua"
)

type LuaBucket struct {
	mu  sync.Mutex
	max int
	vms chan *glua.LState
}

var bucket = LuaBucket{
	mu:  sync.Mutex{},
	max: runtime.NumCPU(),
	vms: make(chan *glua.LState, runtime.NumCPU()),
}

func init() {
	for range bucket.max {
		bucket.vms <- bucket.new()
	}
}

func (pl *LuaBucket) get() *glua.LState {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	return <-pl.vms
}

func (pl *LuaBucket) new() *glua.LState {
	L := glua.NewState(glua.Options{SkipOpenLibs: true})
	return L
}

func (pl *LuaBucket) put(L *glua.LState) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.vms <- L
}

func (pl *LuaBucket) shutdown() {
	for L := range pl.vms {
		L.Close()
	}
}
