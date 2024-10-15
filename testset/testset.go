package testset

import (
	"context"
	"fmt"

	"github.com/canonical/starform/internal"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

type TestSet struct {
	set *starform.ScriptSet
}

func New(options *starform.ScriptSetOptions) (*TestSet, error) {
	set, err := starform.NewScriptSet(options)
	if err != nil {
		return nil, err
	}
	return &TestSet{
		set: set,
	}, nil
}

type testObserver struct {
	fn  func(thread *starlark.Thread, app starlark.Value, event *starform.EventObject)
	set *TestSet
}

var _ starlark.Callable = &testObserver{}
var _ starlark.SafetyAware = &testObserver{}

func (to *testObserver) Freeze()              {}
func (to *testObserver) Name() string         { return "TestObserver" }
func (to *testObserver) String() string       { return "TestObserver" }
func (to *testObserver) Type() string         { return "TestObserver" }
func (to *testObserver) Truth() starlark.Bool { return starlark.True }
func (to *testObserver) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", to.Type())
}
func (to *testObserver) Safety() starlark.SafetyFlags {
	return starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe
}

func (to testObserver) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	to.fn(thread, to.set.set.AppValue, starform.Event(thread))
	return starlark.None, nil
}

func (ts *TestSet) Observe(name string, observer func(thread *starlark.Thread, app starlark.Value, event *starform.EventObject)) {
	if ts.set.EventObservers == nil {
		ts.set.EventObservers = make(map[string][]starlark.Callable)
	}
	ts.set.EventObservers[name] = append(ts.set.EventObservers[name], &testObserver{
		fn:  observer,
		set: ts,
	})
}

func (ts *TestSet) Handle(ctx context.Context, thread *starlark.Thread, event *starform.EventObject) {
	if thread != nil {
		ctx = context.WithValue(ctx, internal.ThreadLocalKey, thread)
	}
	ts.set.Handle(ctx, event)
}
