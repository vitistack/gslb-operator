package checks

import (
	"fmt"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/lua"
	glua "github.com/yuin/gopher-lua"
)

type LuaValidator struct {
	script string
}

func (l *LuaValidator) Validate(resp *http.Response) error {
	vm := lua.Get()
	defer lua.Put(vm)

	vm.SetGlobal("status_code", glua.LNumber(resp.StatusCode))
	vm.SetGlobal("body", glua.LString(""))
	vm.SetGlobal("headers", glua.LString(""))

	vm.DoString(l.script)

	ret := vm.Get(-1)
	if ret == glua.LNil {
		return fmt.Errorf("script returned a nil value")
	}

	if ret == glua.LFalse {
		return fmt.Errorf("health-check failed")
	}

	vm.Pop(1)

	return nil
}
