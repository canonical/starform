package starform

import (
	"context"
	"fmt"
)

type LogLevel int

const (
	_ LogLevel = iota // Placeholder for possible future TraceLevel
	DebugLevel
	PrintLevel
)

const LoadEventName = "<load>"

type LogEntry struct {
	Message   string
	Level     LogLevel
	EventName string
	Path      string
	Line      int32
}

type Logger interface {
	Log(ctx context.Context, entry LogEntry)
}

func (le *LogEntry) String() string {
	if le.Level == DebugLevel {
		return fmt.Sprintf("%s: %s:%d: %s", le.EventName, le.Path, le.Line, le.Message)
	}

	return fmt.Sprintf("%s:%d: %s", le.Path, le.Line, le.Message)
}
