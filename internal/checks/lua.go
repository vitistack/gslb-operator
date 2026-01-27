package checks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/lua"
	glua "github.com/yuin/gopher-lua"
)

var ErrBodyTooBig = errors.New("response body too big")

const maxBodySize = 1 * 1024 * 1024 // 1MB

type LuaValidator struct {
	script   string
	compiled *glua.LFunction
}

// sets global lua values for the script
// executes validation script, and returns the validation result
func (l *LuaValidator) Validate(resp *http.Response) (err error) {
	defer func() { // makes sure we recover from any panics caused by the lua execution
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from lua script validation error: %v", r)
		}
	}()
	vm := lua.Get()
	defer lua.Put(vm)

	sandbox := lua.GetSandBox(vm)

	if l.compiled == nil {
		// compile user script
		// and apply sandbox environment for execution
		userScript, err := vm.LoadString(l.script)
		if err != nil {
			return fmt.Errorf("could not compile user-script: %w", err)
		}
		l.compiled = userScript
	}

	// populate response-body
	luaBody, err := l.readBody(resp.Body)
	if err != nil {
		bslog.Error("unable to load response body", slog.String("reason", err.Error()))
	}

	// create lua table for header values
	luaHeaders := vm.NewTable()
	for key, val := range resp.Header {
		if len(val) > 0 {
			luaHeaders.RawSetString(key, glua.LString(val[0]))
		}
	}

	// set sandbox values
	sandbox.RawSetString("status_code", glua.LNumber(resp.StatusCode))
	sandbox.RawSetString("body", glua.LString(luaBody))
	sandbox.RawSetString("headers", luaHeaders)

	// set execution timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*150)
	defer cancel()
	vm.SetContext(ctx)

	vm.SetFEnv(l.compiled, sandbox)
	vm.Push(l.compiled)
	if err := vm.PCall(0, 1, nil); err != nil {
		// Clean up before returning
		sandbox.RawSetString("status_code", glua.LNil)
		sandbox.RawSetString("body", glua.LNil)
		sandbox.RawSetString("headers", glua.LNil)
		return fmt.Errorf("lua script execution failed: %w", err)
	}

	// Get the return value
	ret := vm.Get(-1)
	vm.Pop(1)

	// Clean up sandbox
	sandbox.RawSetString("status_code", glua.LNil)
	sandbox.RawSetString("body", glua.LNil)
	sandbox.RawSetString("headers", glua.LNil)

	if ret == glua.LNil {
		return fmt.Errorf("script returned a nil value")
	}

	if ret == glua.LFalse {
		return fmt.Errorf("health-check validation returned a false value")
	}

	return err
}

// reads the response body into a string representation
func (l *LuaValidator) readBody(body io.ReadCloser) (string, error) {
	unableToLoad := "unable-to-load"
	reader := io.LimitReader(body, maxBodySize)

	rawBody, err := io.ReadAll(reader)
	if err != nil {
		return unableToLoad, fmt.Errorf("could not read response body: %w", err)
	}

	if len(rawBody) == maxBodySize { // we hit the limit, try to read more
		extra := make([]byte, 1)
		n, _ := body.Read(extra)
		if n > 0 {
			return unableToLoad, fmt.Errorf("%w: response body exceeded maximum body size of %d bytes", ErrBodyTooBig, maxBodySize)
		}
	}

	return string(rawBody), nil
}
