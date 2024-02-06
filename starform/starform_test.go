package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type testSourceLoader struct {
	sources map[string][]testSource
}

var _ starform.ScriptletLoader = &testSourceLoader{}

func (tsl *testSourceLoader) Load(nameOrPath string) ([]starform.ScriptletSource, error) {
	if sources, ok := tsl.sources[nameOrPath]; ok {
		result := make([]starform.ScriptletSource, len(sources))
		for i := range sources {
			result[i] = &sources[i]
		}
		return result, nil
	}
	return nil, fmt.Errorf("not found")
}

type testSource struct {
	name, content string
	hash          interface{}
}

var _ starform.ScriptletSource = &testSource{}

func (ts *testSource) Path() string                  { return ts.name }
func (ts *testSource) Content() (interface{}, error) { return startest.Reindent(ts.content) }
func (ts *testSource) Hash() (interface{}, error)    { return ts.hash, nil }

func TestRunSimpleScriptlet(t *testing.T) {
	tests := []struct {
		name        string
		sources     []testSource
		expectedLog string
	}{{
		name:        "empty",
		sources:     nil,
		expectedLog: "",
	}, {
		name: "no-init (single)",
		sources: []testSource{{
			name: "single.star",
			content: `
				print("single")
				def never_called():
					print("never-called")
			`,
			hash: 1,
		}},
		expectedLog: "single\n",
	}, {
		name: "no-init (multiple)",
		sources: []testSource{{
			name: "second.star",
			content: `
				print("second")
				def never_called():
					print("never-called")
			`,
			hash: 1,
		}, {
			name: "first.star",
			content: `
				print("first")
				def never_called():
					print("never-called")
			`,
			hash: 2,
		}},
		expectedLog: "first\nsecond\n",
	}, {
		name: "init (single)",
		sources: []testSource{{
			name: "single.star",
			content: `
				print("Hello,")
				def init():
					print("world!")
			`,
			hash: 1,
		}},
		expectedLog: "Hello,\nworld!\n",
	}, {
		name: "init (multiple)",
		sources: []testSource{{
			name: "lib.star",
			content: `
					print("second")
					def never_called():
						print("never-called")
				`,
			hash: 1,
		}, {
			name: "init.star",
			content: `
					def init():
						print("third")
					print("first")
				`,
			hash: 4,
		}},
		expectedLog: "first\nsecond\nthird\n",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &strings.Builder{}
			printHandler := func(thread *starlark.Thread, msg string) {
				log.WriteString(msg)
				log.WriteByte('\n')
			}
			opts := &starform.ExtensionSetOptions{
				PrintHandler: printHandler,
				Loader: &testSourceLoader{
					sources: map[string][]testSource{
						"test": test.sources,
					},
				},
			}
			set, err := starform.NewExtensionSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			extension, err := set.Load("test")
			if err != nil {
				t.Fatal(err)
			}
			if extension == nil {
				t.Fatalf("extension should not be nil")
			}
			if extension.Name != "test" {
				t.Errorf("extension name mismatch: expected %v got %v", "test", extension.Name)
			}
			if actualLog := log.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v go %v", test.expectedLog, actualLog)
			}
		})
	}
}

func TestOnlyStarScriptlets(t *testing.T) {
	opts := &starform.ExtensionSetOptions{
		PrintHandler: func(thread *starlark.Thread, msg string) {},
		Loader: &testSourceLoader{
			sources: map[string][]testSource{
				"test": {{
					name:    "lib.html",
					content: `<b>not relevant</b>`,
					hash:    123,
				}},
			},
		},
	}
	set, err := starform.NewExtensionSet(opts)
	if err != nil {
		t.Fatal(err)
	}
	_, err = set.Load("test")
	if err == nil {
		t.Error("expected error")
	}
}
