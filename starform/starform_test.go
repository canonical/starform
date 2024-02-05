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

var _ starform.ScriptLoader = &testSourceLoader{}

func (tsl *testSourceLoader) Load(nameOrPath string) ([]starform.ScriptSource, error) {
	if sources, ok := tsl.sources[nameOrPath]; ok {
		result := make([]starform.ScriptSource, len(sources))
		for i := range sources {
			result[i] = &sources[i]
		}
		return result, nil
	}
	return nil, fmt.Errorf("not found")
}

type testSource struct {
	name, content, hash string
}

var _ starform.ScriptSource = &testSource{}

func (ts *testSource) Name() string                  { return ts.name }
func (ts *testSource) Content() (interface{}, error) { return startest.Reindent(ts.content) }
func (ts *testSource) Hash() interface{}             { return ts.hash }

func TestRunSimpleScriptlet(t *testing.T) {
	builder := &strings.Builder{}
	set, err := starform.NewExtensionSet(starform.ExtensionSetOptions{
		PrintHandler: func(thread *starlark.Thread, msg string) {
			builder.WriteString(msg)
			builder.WriteByte('\n')
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
					hash: "edb7d785abbf5ffa17675d0da845f1debe38fc68f94f4726599fc2189c181eea",
				}, {
					name: "init.star",
					content: `
						def init():
							print("third")
						print("first")
					`,
					hash: "7377d1f78160ca48dabe25115ece825bd6e5168baa9ab036aa0eef3eba608ba2",
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	scriptlet, err := set.Load("test")
	if err != nil {
		t.Fatal(err)
	}

	if scriptlet == nil {
		t.Error("scriptlet should not be nil")
	}
	if scriptlet.Name != "test" {
		t.Errorf("scriptlet name mismatch: expected %v got %v", "test", scriptlet.Name)
	}
	const expectedLog = "first\nsecond\nthird\n"
	if actualLog := builder.String(); actualLog != expectedLog {
		t.Errorf("output error: expected %v go %v", expectedLog, actualLog)
	}
}
