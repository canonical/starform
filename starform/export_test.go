package starform

import (
	"context"

	"github.com/canonical/starlark/starlark"
)

type TestCacheBase struct{}

func (*TestCacheBase) private() {}

const LoadEventName = loadEventName

func WithParentThread(ctx context.Context, thread *starlark.Thread) context.Context {
	return context.WithValue(ctx, threadLocalKey, thread)
}

func (app *AppObject) Value() starlark.HasSafeAttrs {
	return app.value()
}

func InitState() interface{} {
	return &initState{
		eventObservers: make(map[string][]starlark.Callable),
	}
}

var PrintBuiltin = printBuiltin
var DebugBuiltin = debugBuiltin

func NewScriptLogger(logger Logger, path string) starlark.Value {
	return &scriptLogger{
		logger: logger,
		path:   path,
	}
}

var AfterFunc = afterFunc
