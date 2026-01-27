package lua

import (
	"fmt"

	glua "github.com/yuin/gopher-lua"
)

type SandboxConfig glua.LTable

var sandBox *SandboxConfig

func LoadSandboxConfig(filename string) error {
	vm := glua.NewState()
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

	for range bucket.max {
		bucket.vms <- bucket.new()
	}

	return nil
}

// get sandbox environment for VM
func GetSandBox(vm *glua.LState) *glua.LTable {
	env := vm.GetGlobal("sandbox")

	if sandbox, ok := env.(*glua.LTable); ok {
		return sandbox
	}
	return nil
}

func Get() *glua.LState {
	return bucket.get()
}

func Put(L *glua.LState) {
	bucket.put(L)
}

func Shutdown() {
	bucket.shutdown()
}
