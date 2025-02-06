package formtest_test

import (
	"testing"

	"github.com/canonical/starform/formtest"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

func TestExampleRunString(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(&starform.AppObject{
		Name: "test",
	})
	ft.SetEvent(&starform.EventObject{
		Name: "event",
	})
	ft.RunString(`
		def init():
			test.observe('event', on_event)
		def on_event(event):
			assert.fails(lambda: test.observe('event', on_event), 'unavailable')
	`)
}

type FooState struct {
	Intents []Intent
}

var app = &starform.AppObject{
	Name: "bar",

	Methods: []*starlark.Builtin{
		starlark.NewBuiltinWithSafety("add_intent", addIntentSafety, bar_add_intent),
	},
}

const addIntentSafety = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

type Intent struct {
	args starlark.Tuple
	_    [1024]byte // Simulate other fields.
}

func bar_add_intent(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	event := starform.Event(thread)
	if event.Name != "foo" {
		return nil, starform.ErrUnavailable
	}

	state := event.State.(*FooState)
	for _, intent := range args {
		intent.Freeze()
	}
	if err := thread.AddAllocs(starlark.EstimateSize(Intent{})); err != nil {
		return nil, err
	}
	intentsAppender := starlark.NewSafeAppender(thread, &state.Intents)
	if err := intentsAppender.Append(Intent{args: args}); err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func TestExampleRunStringFunctionality(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(app)
	ft.SetEvent(&starform.EventObject{
		Name:  "foo",
		State: &FooState{},
	})
	ft.RunString(`
		def init():
			bar.observe('foo', on_foo)
		def on_foo(event):
			ret = bar.add_intent('sudo make me a sandwich')
			assert.eq(ret, None)
	`)
}

func TestExampleRunStringAllocs(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(app)
	ft.SetEvent(&starform.EventObject{
		Name:  "bar",
		State: &FooState{},
	})
	ft.RequireSafety(starlark.MemSafe)
	ft.RunString(`
		def init():
			bar.observe('foo', on_foo)
		def on_foo(event):
			for _ in ft.ntimes():
				bar.add_intent('sudo make me a sandwich')
	`)
}

func TestExampleRunThreadFunctionality(t *testing.T) {
	fooState := &FooState{}

	ft := formtest.From(t)
	ft.SetApp(app)
	ft.SetEvent(&starform.EventObject{
		Name:  "foo",
		State: fooState,
	})
	ft.RunThread(func(thread *starlark.Thread) {
		previousNumIntents := len(fooState.Intents)

		bar_add_intent_builtin := starlark.NewBuiltinWithSafety("add_intent", addIntentSafety, bar_add_intent)
		ret, err := starlark.Call(thread, bar_add_intent_builtin, starlark.Tuple{starlark.None}, nil)
		if err != nil {
			ft.Error(err)
		}
		if ret != starlark.None {
			ft.Errorf("expected None got: %v", ret)
		}
		if expectedIntents := previousNumIntents + 1; len(fooState.Intents) != expectedIntents {
			ft.Errorf("expected %d total intents, got %d", expectedIntents, len(fooState.Intents))
		}
	})
}
