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

const debugBuiltinSafety = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

var debugBuiltin = starlark.NewBuiltinWithSafety(
	"debug",
	debugBuiltinSafety,
	func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if thread.Print == nil {
			return starlark.None, nil
		}
		// Early stop if calling Print would cause a safety violation
		if err := starlark.CheckSafety(thread, thread.PrintSafety); err != nil {
			return nil, err
		}

		sep := " "
		if err := starlark.UnpackArgs(b.Name(), nil, kwargs, "sep?", &sep); err != nil {
			return nil, err
		}

		buf := starlark.NewSafeStringBuilder(thread)
		for i, v := range args {
			if i > 0 {
				if _, err := buf.WriteString(sep); err != nil {
					return nil, err
				}
			}
			if s, ok := starlark.AsString(v); ok {
				if _, err := buf.WriteString(s); err != nil {
					return nil, err
				}
			} else if b, ok := v.(starlark.Bytes); ok {
				if _, err := buf.WriteString(string(b)); err != nil {
					return nil, err
				}
			} else if stringer, ok := v.(starlark.SafeStringer); ok {
				if err := stringer.SafeString(thread, buf); err != nil {
					return nil, err
				}
			} else {
				if err := starlark.CheckSafety(thread, starlark.NotSafe); err != nil {
					return nil, err
				}
				buf.WriteString(v.String())
			}
		}

		thread.Print(thread, buf.String())
		return starlark.None, nil
	})
