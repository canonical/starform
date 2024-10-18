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
		starlark.NewBuiltinWithSafety("add_intent", addIntentSafety, foo_add_intent),
	},
}

const addIntentSafety = starlark.MemSafe | starlark.CPUSafe | starlark.TimeSafe | starlark.IOSafe

type Intent struct {
	args    starlark.Tuple
	padding [1024]byte // Simulate other fields.
}

func foo_add_intent(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

func TestExampleRunStringForFunctionality(t *testing.T) {
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

func TestExampleRunStringForAllocs(t *testing.T) {
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
			for _ in st.ntimes():
				bar.add_intent('sudo make me a sandwich')
	`)
}

func TestExampleRunThreadForFunctionality(t *testing.T) {
	fooState := &FooState{}

	ft := formtest.From(t)
	ft.SetApp(app)
	ft.SetEvent(&starform.EventObject{
		Name:  "foo",
		State: fooState,
	})
	ft.RunThread(func(thread *starlark.Thread) {
		st := ft.ST()
		app := ft.MakeAppValue().(starlark.HasSafeAttrs)
		fn, _ := app.SafeAttr(thread, "add_intent")
		if fn == nil {
			ft.Fatal("no such method: add_intent")
		}

		previousNumIntents := len(fooState.Intents)
		ret, err := starlark.Call(thread, fn, starlark.Tuple{starlark.None}, nil)
		if err != nil {
			st.Error(err)
		}
		if ret != starlark.None {
			st.Errorf("expected None got: %v", ret)
		}
		if expectedIntents := previousNumIntents + 1; len(fooState.Intents) != expectedIntents {
			st.Errorf("expected %d total intents: got %d", expectedIntents, len(fooState.Intents))
		}
	})
}

func TestExampleRunThreadForAllocs(t *testing.T) {
	ft := formtest.From(t)
	ft.SetApp(app)
	ft.SetEvent(&starform.EventObject{
		Name:  "foo",
		State: &FooState{},
	})
	ft.RequireSafety(starlark.MemSafe)
	ft.RunThread(func(thread *starlark.Thread) {
		st := ft.ST()
		app := ft.MakeAppValue().(starlark.HasSafeAttrs)
		fn, _ := app.SafeAttr(thread, "add_intent")
		if fn == nil {
			ft.Fatal("no such method: add_intent")
		}
		args := starlark.Tuple{starlark.None}
		for i := 0; i < st.N; i++ {
			ret, err := starlark.Call(thread, fn, args, nil)
			if err != nil {
				st.Error(err)
			}
			st.KeepAlive(ret)
		}
	})
}
