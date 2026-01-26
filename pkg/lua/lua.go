package lua

import (
	glua "github.com/yuin/gopher-lua"
)

func Get() *glua.LState{
	return bucket.get()
}

func Put(L *glua.LState) {
	bucket.put(L)
}

func Shutdown() {
	bucket.shutdown()
}