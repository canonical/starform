package testset

import (
	"fmt"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

type TestSet struct {
	*starform.ScriptSet
}

func New(options *starform.ScriptSetOptions) (*TestSet, error) {
	set, err := starform.NewScriptSet(options)
	if err != nil {
		return nil, err
	}

	return &TestSet{
		ScriptSet: set,
	}, nil
}

type testObserver struct {
	fn  func(thread *starlark.Thread, app starlark.Value, event *starform.EventObject)
	set *TestSet
}

func (to *testObserver) Freeze()              {}
func (to *testObserver) Name() string         { return "TestObserver" }
func (to *testObserver) String() string       { return "TestObserver" }
func (to *testObserver) Type() string         { return "TestObserver" }
func (to *testObserver) Truth() starlark.Bool { return starlark.True }
func (to *testObserver) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", to.Type())
}

func (to testObserver) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	to.fn(thread, to.set.AppValue, starform.Event(thread))
	return starlark.None, nil
}

var _ starlark.Callable = &testObserver{}

func (ts *TestSet) Observe(name string, observer func(thread *starlark.Thread, app starlark.Value, event *starform.EventObject)) {
	if ts.EventObservers == nil {
		ts.EventObservers = make(map[string][]starlark.Callable)
	}
	ts.EventObservers[name] = append(ts.EventObservers[name], &testObserver{
		fn:  observer,
		set: ts,
	})
}
