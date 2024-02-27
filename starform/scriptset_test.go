package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type testScriptLoader struct {
	sources map[string][]testScriptSource
}

var _ starform.ScriptLoader = &testScriptLoader{}

func (tel *testScriptLoader) Load(name string) ([]starform.ScriptSource, error) {
	if sources, ok := tel.sources[name]; ok {
		result := make([]starform.ScriptSource, len(sources))
		for i := range sources {
			result[i] = &sources[i]
		}
		return result, nil
	}
	return nil, fmt.Errorf("not found")
}

type testScriptSource struct {
	name, content string
}

var _ starform.ScriptSource = &testScriptSource{}

func (tes *testScriptSource) Path() string { return tes.name }
func (tes *testScriptSource) Content() ([]byte, error) {
	content, err := startest.Reindent(tes.content)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func TestOptionsValidation(t *testing.T) {
	tests := []struct {
		name     string
		opts     *starform.ScriptSetOptions
		expected string
	}{{
		name:     "no loader",
		opts:     &starform.ScriptSetOptions{},
		expected: "Loader cannot be nil",
	}, {
		name: "NotSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			Loader: &testScriptLoader{},
		},
	}, {
		name: "MemSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			Loader:         &testScriptLoader{},
			RequiredSafety: starlark.MemSafe,
		},
		expected: "cannot run starlark with unbounded MaxAllocs",
	}, {
		name: "MemSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			Loader:         &testScriptLoader{},
			RequiredSafety: starlark.MemSafe,
			MaxAllocs:      100,
		},
	}, {
		name: "CPUSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			Loader:         &testScriptLoader{},
			RequiredSafety: starlark.CPUSafe,
		},
		expected: "cannot run starlark with unbounded MaxSteps",
	}, {
		name: "CPUSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			Loader:         &testScriptLoader{},
			RequiredSafety: starlark.CPUSafe,
			MaxSteps:       100,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.opts.CheckValid()
			if test.expected == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			} else if test.expected != "" && err == nil {
				t.Error("expected error")
			} else if test.expected != "" && err.Error() != test.expected {
				t.Errorf("expected %v got %v", test.expected, err)
			}
		})
	}
}

func TestLoadSimpleScriptSet(t *testing.T) {
	tests := []struct {
		name        string
		sources     []testScriptSource
		expectedLog string
	}{{
		name:        "empty",
		sources:     nil,
		expectedLog: "",
	}, {
		name: "no-init (single)",
		sources: []testScriptSource{{
			name: "single.star",
			content: `
				print("single")
				def do_not_call():
					fail('unexpectedly called')
			`,
		}},
		expectedLog: "single\n",
	}, {
		name: "no-init (multiple)",
		sources: []testScriptSource{{
			name: "2.star",
			content: `
				print("second")
				def do_not_call():
					fail('unexpectedly called')
			`,
		}, {
			name: "1.star",
			content: `
				print("first")
				def do_not_call():
					fail('unexpectedly called')
			`,
		}},
		expectedLog: "first\nsecond\n",
	}, {
		name: "init (single)",
		sources: []testScriptSource{{
			name: "single.star",
			content: `
				print("Hello,")
				def init():
					print("world!")
			`,
		}},
		expectedLog: "Hello,\nworld!\n",
	}, {
		name: "init (multiple)",
		sources: []testScriptSource{{
			name: "lib.star",
			content: `
				print("second")
				def do_not_call():
					fail('unexpectedly called')
			`,
		}, {
			name: "init.star",
			content: `
				def init():
					print("third")
				print("first")
			`,
		}},
		expectedLog: "first\nsecond\nthird\n",
	}, {
		name: "unsupported extensions",
		sources: []testScriptSource{{
			name: "index.html",
			content: `
				<b>not a script</b>
			`,
		}, {
			name: "lib.star.swp",
			content: `
				print("vim cache file")
			`,
		}, {
			name: "README.md",
			content: `
				# Docs
			`,
		}},
		expectedLog: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &strings.Builder{}
			printHandler := func(thread *starlark.Thread, msg string) {
				log.WriteString(msg)
				log.WriteByte('\n')
			}
			loader := &testScriptLoader{
				sources: map[string][]testScriptSource{
					"test": test.sources,
				},
			}
			opts := &starform.ScriptSetOptions{
				PrintHandler: printHandler,
				Loader:       loader,
			}
			err := opts.CheckValid()
			if err != nil {
				t.Fatal(err)
			}
			set, err := starform.NewScriptSet(opts, "test")
			if err != nil {
				t.Fatal(err)
			}
			if set == nil {
				t.Fatalf("extension should not be nil")
			}
			if set.Name != "test" {
				t.Errorf("extension name mismatch: expected %v got %v", "test", set.Name)
			}
			if actualLog := log.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v go %v", test.expectedLog, actualLog)
			}
		})
	}
}
