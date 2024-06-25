package starform

import (
	"context"
	"fmt"

	"github.com/canonical/starlark/starlark"
)

type Logger interface {
	Log(ctx context.Context, entry LogEntry)
}

type LogLevel int

const (
	_ LogLevel = iota // Placeholder for possible future TraceLevel
	DebugLevel
	PrintLevel
)

type LogEntry struct {
	Message   string
	Level     LogLevel
	EventName string
	Path      string
	Line      int32
}

func (le *LogEntry) String() string {
	if le.Level == DebugLevel {
		return fmt.Sprintf("%s: %s:%d: %s", le.EventName, le.Path, le.Line, le.Message)
	}

	return fmt.Sprintf("%s:%d: %s", le.Path, le.Line, le.Message)
}

type scriptLogger struct {
	logger Logger
	path   string
}

var _ starlark.Value = &scriptLogger{}

func (sl *scriptLogger) String() string       { return "<scriptLogger>" }
func (sl *scriptLogger) Type() string         { return "scriptLogger" }
func (sl *scriptLogger) Freeze()              {}
func (sl *scriptLogger) Truth() starlark.Bool { return true }
func (sl *scriptLogger) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", sl.Type())
}

func (sl *scriptLogger) log(thread *starlark.Thread, msg string, level LogLevel) {
	if sl.logger == nil {
		return
	}

	line := int32(0)
	if thread.CallStackDepth() > 1 {
		callerFrame := thread.CallFrame(1)
		line = callerFrame.Pos.Line
	}

	event := Event(thread)

	sl.logger.Log(thread.Context(), LogEntry{
		Message:   msg,
		Level:     level,
		EventName: event.Name,
		Path:      sl.path,
		Line:      line,
	})
}

const logSafety = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

var debugBuiltin = starlark.NewBuiltinWithSafety("debug", logSafety, debug)
var printBuiltin = starlark.NewBuiltinWithSafety("print", logSafety, print)

func print(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return log(thread, b, args, kwargs, PrintLevel)
}

func debug(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return log(thread, b, args, kwargs, DebugLevel)
}

func log(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, level LogLevel) (starlark.Value, error) {
	recv := b.Receiver().(*scriptLogger)

	sep := " "
	if err := starlark.UnpackArgs(b.Name(), nil, kwargs, "sep?", &sep); err != nil {
		return nil, err
	}

	msg, err := formatMessage(thread, sep, args)
	if err != nil {
		return nil, err
	}

	recv.log(thread, msg, level)
	return starlark.None, nil
}

func formatMessage(thread *starlark.Thread, sep string, values starlark.Tuple) (string, error) {
	sb := starlark.NewSafeStringBuilder(thread)
	for i, v := range values {
		if i > 0 {
			if _, err := sb.WriteString(sep); err != nil {
				return "", err
			}
		}
		if s, ok := starlark.AsString(v); ok {
			// Avoid quoted v.SafeString()/v.String() representation.
			if _, err := sb.WriteString(s); err != nil {
				return "", err
			}
		} else if b, ok := v.(starlark.Bytes); ok {
			// Avoid quoted v.SafeString()/v.String() representation.
			if _, err := sb.WriteString(string(b)); err != nil {
				return "", err
			}
		} else if stringer, ok := v.(starlark.SafeStringer); ok {
			if err := stringer.SafeString(thread, sb); err != nil {
				return "", err
			}
		} else {
			if err := starlark.CheckSafety(thread, starlark.NotSafe); err != nil {
				return "", err
			}
			if _, err := sb.WriteString(v.String()); err != nil {
				return "", err
			}
		}
	}

	return sb.String(), nil
}
