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
	app := &starform.AppObject{
		Name: "test",
	}
	fooModule := &starlarkstruct.Module{
		Name: "foo",
		Members: starlark.StringDict{
			"bar": starlark.String("baz"),
		},
	}

	t.Run("loadable", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: app,
			Modules: []starform.Module{&internal.SystemModule{
				Module: fooModule,
			}},
		})
		if err != nil {
			t.Error(err)
		}

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					load('foo', 'bar')

					def init():
						if bar != 'baz':
							fail('foo has incorrect bar field: expected %r, got %r', 'baz', bar)
				`,
			},
		})
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("predeclared", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: app,
			Modules: []starform.Module{&internal.SystemModule{
				Module:      fooModule,
				Predeclared: true,
			}},
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

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					load('foo', 'bar')
				`,
			},
		})
		if err == nil {
			t.Error("expected error, got success")
		}
	})

	t.Run("valid-custom", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: app,
			Modules: []starform.Module{&testModule{
				name:    "test/foo",
				members: fooModule.Members,
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

	t.Run("invalid-custom", func(t *testing.T) {
		tests := []struct {
			name       string
			moduleName string
		}{{
			name:       "missing-app-name-prefix",
			moduleName: "nonstandard",
		}, {
			name:       "with-file-extension",
			moduleName: "test/foo.star",
		}, {
			name:       "nonstandard-characters",
			moduleName: "ඞ",
		}}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := starform.NewScriptSet(&starform.ScriptSetOptions{
					App: &starform.AppObject{
						Name: "test",
					},
					Modules: []starform.Module{&testModule{
						name:    test.moduleName,
						members: starlark.StringDict{},
					}},
				})
				if err == nil {
					t.Error("expected error, got success")
				}
			})
		}
	})
}
