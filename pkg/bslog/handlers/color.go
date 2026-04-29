package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

const (
	ansiReset   = "\033[0m"
	ansiWhite   = "\033[37m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiRed     = "\033[31m"
	ansiBoldRed = "\033[1;31m"
	ansiMagenta = "\033[35m"
)

// ColorHandler wraps a base slog.Handler and colors the output based on loglevel
type ColorHandler struct {
	out  io.Writer
	base slog.Handler
	buf  *bytes.Buffer
	mu   *sync.Mutex
}

// NewColorHandler builds a ColorHandler. The base handler is reconfigured to
// write into an internal buffer; pass a factory that builds a base handler for
// the given writer (e.g. slog.NewTextHandler).
func NewColorHandler(out io.Writer, factory func(io.Writer) slog.Handler) slog.Handler {
	if out == nil {
		out = os.Stdout
	}

	buf := &bytes.Buffer{}
	return &ColorHandler{
		out:  out,
		buf:  buf,
		base: factory(buf),
		mu:   &sync.Mutex{},
	}
}

func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *ColorHandler) Handle(ctx context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buf.Reset()
	if err := h.base.Handle(ctx, record); err != nil {
		return err
	}

	line := h.buf.Bytes()
	line = colorByLevel(line, record.Level)

	_, err := h.out.Write(line)
	return err
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		out:  h.out,
		buf:  h.buf,
		base: h.base.WithAttrs(attrs),
		mu:   h.mu,
	}
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		out:  h.out,
		buf:  h.buf,
		base: h.base.WithGroup(name),
		mu:   h.mu,
	}
}

// colorizeLevel finds the level token in the formatted line and wraps it
// with ANSI color codes. Supports both text (`level=INFO`) and JSON
// (`"level":"INFO"`) encodings.
func colorByLevel(line []byte, lvl slog.Level) []byte {
	code := levelColor(lvl)
	if code == "" {
		return line
	}

	line = wrap(line, 0, len(line), code)

	return line
}

func wrap(line []byte, start, end int, code string) []byte {
	out := make([]byte, 0, len(line)+len(code)+len(ansiReset))
	out = append(out, line[:start]...)
	out = append(out, code...)
	out = append(out, line[start:end]...)
	out = append(out, ansiReset...)
	out = append(out, line[end:]...)
	return out
}

func levelColor(l slog.Level) string {
	switch {
	case l > slog.LevelError:
		return ansiBoldRed
	case l >= slog.LevelError:
		return ansiRed
	case l >= slog.LevelWarn:
		return ansiYellow
	case l >= slog.LevelInfo:
		return ansiGreen
	case l >= slog.LevelDebug:
		return ansiWhite
	default:
		return ansiMagenta
	}
}
