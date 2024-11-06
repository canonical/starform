package starform_test

import (
	"context"
	"testing"

	"github.com/canonical/starform/internal"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
)

type customTestModule struct {
	name    string
	members starlark.StringDict
}

func (ctm *customTestModule) Name() string                 { return ctm.name }
func (ctm *customTestModule) Members() starlark.StringDict { return ctm.members }

func TestModules(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}
	testModule := &starlarkstruct.Module{
		Name: "test_module",
		Members: starlark.StringDict{
			"module_value": starlark.String("foo"),
		},
	}

	t.Run("loadable", func(t *testing.T) {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: app,
			Modules: []starform.Module{&internal.SystemModule{
				Module: testModule,
			}},
		})
		if err != nil {
			t.Error(err)
		}

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					load('test_module', 'module_value')

					def init():
						if module_value != 'foo':
							fail('test_module has incorrect module_value field: expected %r, got %r' % ('foo', module_value))
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
				Module:      testModule,
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
						if module_value != 'foo':
							fail('test_module has incorrect module_value field: expected %r, got %r' % ('foo', module_value))
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
					load('test_module', 'module_value')
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
			Modules: []starform.Module{&customTestModule{
				name:    "test/test_module",
				members: testModule.Members,
			}},
		})
		if err != nil {
			t.Error(err)
		}

		err = set.LoadSources(context.Background(), []starform.ScriptSource{
			&testScriptSource{
				name: "test.star",
				content: `
					load('test/test_module', 'module_value')

					def init():
						if module_value != 'foo':
							fail("expected %r, got %r" % ('foo', module_value))
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
					Modules: []starform.Module{&customTestModule{
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
