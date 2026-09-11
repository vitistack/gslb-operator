package lua

import (
	"fmt"
	"sync"

	glua "github.com/yuin/gopher-lua"
)

type SandboxConfig glua.LTable

var sandBox *SandboxConfig
var vmPool = sync.Pool{
	New: func() any { return newLuaState() },
}

func LoadSandboxConfig(filename string) error {
	vm := glua.NewState() // lua state used to build the sandbox
	defer vm.Close()

	if err := vm.DoFile(filename); err != nil {
		return fmt.Errorf("unable to execute configuration file: %w", err)
	}

	// Validate sandbox_env exists
	envValue := vm.GetGlobal("env")
	if envValue.Type() != glua.LTTable {
		return fmt.Errorf("sandbox_env must be a table")
	}

	sandBox = (*SandboxConfig)(envValue.(*glua.LTable))

	return nil
}

func NewRequestEnv(vm *glua.LState) *glua.LTable {
	env := vm.NewTable()
	mt := vm.NewTable()
	mt.RawSetString("__index", (*glua.LTable)(sandBox))
	vm.SetMetatable(env, mt)
	return env
}

func Get() *glua.LState {
	return vmPool.Get().(*glua.LState)
}

func Put(luaState *glua.LState) {
	luaState.SetTop(0)
	vmPool.Put(luaState)
}

func newLuaState() *glua.LState {
	L := glua.NewState(glua.Options{
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
		MinimizeStackMemory: true,
		CallStackSize:       64,
	})
	L.SetGlobal("sandbox", (*glua.LTable)(sandBox))
	return L
}
