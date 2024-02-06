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
func (ts *testSource) Hash() interface{}             { return ts.hash }

func TestRunSimpleScriptlet(t *testing.T) {
	log := &strings.Builder{}

	opts := &starform.ExtensionSetOptions{
		PrintHandler: func(thread *starlark.Thread, msg string) {
			log.WriteString(msg)
			log.WriteByte('\n')
		},
		Loader: &testSourceLoader{
			sources: map[string][]testSource{
				"test": {{
					name: "lib.star",
					content: `
						print("second")
						def never_called():
							print("never-called")
					`,
					hash: 123,
				}, {
					name: "init.star",
					content: `
						def init():
							print("third")
						print("first")
					`,
					hash: 456,
				}},
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

	const expectedLog = "first\nsecond\nthird\n"
	if actualLog := log.String(); actualLog != expectedLog {
		t.Errorf("output error: expected %v go %v", expectedLog, actualLog)
	}
}
