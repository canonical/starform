package internal

import "github.com/canonical/starlark/starlark"

type ScriptSetBase struct {
	EventObservers map[string][]starlark.Callable
	AppValue       starlark.Value
}

const ThreadLocalKey = "starform-thread"
