package starform_test

import (
	"context"
	"testing"

	"github.com/canonical/starform/internal"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
)

type testModule struct {
	name    string
	members starlark.StringDict
}

func (tm *testModule) Name() string                 { return tm.name }
func (tm *testModule) Members() starlark.StringDict { return tm.members }

func TestModules(t *testing.T) {
	t.Run("override-std", func(t *testing.T) {
		_, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "test",
			},
			Modules: []starform.Module{&testModule{
				name:    "foo",
				members: starlark.StringDict{},
			}},
		})
		if err == nil {
			t.Error("expected err got success")
		}
	})

	t.Run("std", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "test",
			},
			Modules: []starform.Module{
				internal.NewPredeclared(&starlarkstruct.Module{
					Name: "foo",
					Members: starlark.StringDict{
						"bar": starlark.String("baz"),
					},
				}),
			},
		})
		if err != nil {
			t.Error(err)
		}

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					def init():
						if foo.bar != 'baz':
							fail('foo has incorrect bar field: expected %r, got %r', 'baz', foo.bar)
				`,
			},
		})
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("custom", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "test",
			},
			Modules: []starform.Module{&testModule{
				name: "test/foo",
				members: starlark.StringDict{
					"bar": starlark.String("baz"),
				},
			}},
		})
		if err != nil {
			t.Error(err)
		}

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					load('test/foo', 'bar')

					def init():
						if bar != 'baz':
							fail("expected 'baz', got: ", bar)
				`,
			},
		})
		if err != nil {
			t.Error(err)
		}
	})
}
