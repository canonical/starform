package starform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type testExtensionLoader struct {
	sources map[string][]testExtensionSource
}

var _ starform.ExtensionLoader = &testExtensionLoader{}

func (tel *testExtensionLoader) Load(nameOrPath string) ([]starform.ExtensionSource, error) {
	if sources, ok := tel.sources[nameOrPath]; ok {
		result := make([]starform.ExtensionSource, len(sources))
		for i := range sources {
			result[i] = &sources[i]
		}
		return result, nil
	}
	return nil, fmt.Errorf("not found")
}

type testExtensionSource struct {
	name, content string
}

var _ starform.ExtensionSource = &testExtensionSource{}

func (tes *testExtensionSource) Path() string { return tes.name }
func (tes *testExtensionSource) Content() ([]byte, error) {
	content, err := startest.Reindent(tes.content)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func TestOptionsValidation(t *testing.T) {
	tests := []struct {
		name     string
		opts     *starform.ExtensionSetOptions
		expected string
	}{{
		name:     "no loader",
		opts:     &starform.ExtensionSetOptions{},
		expected: "Loader cannot be nil",
	}, {
		name: "NotSafe (unbounded)",
		opts: &starform.ExtensionSetOptions{
			Loader: &testExtensionLoader{},
		},
	}, {
		name: "MemSafe (unbounded)",
		opts: &starform.ExtensionSetOptions{
			Loader:         &testExtensionLoader{},
			RequiredSafety: starlark.MemSafe,
		},
		expected: "cannot run starlark with unbounded MaxAllocs",
	}, {
		name: "MemSafe (bounded)",
		opts: &starform.ExtensionSetOptions{
			Loader:         &testExtensionLoader{},
			RequiredSafety: starlark.MemSafe,
			MaxAllocs:      100,
		},
	}, {
		name: "CPUSafe (unbounded)",
		opts: &starform.ExtensionSetOptions{
			Loader:         &testExtensionLoader{},
			RequiredSafety: starlark.CPUSafe,
		},
		expected: "cannot run starlark with unbounded MaxSteps",
	}, {
		name: "CPUSafe (bounded)",
		opts: &starform.ExtensionSetOptions{
			Loader:         &testExtensionLoader{},
			RequiredSafety: starlark.CPUSafe,
			MaxSteps:       100,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := starform.NewExtensionSet(test.opts)
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

func TestLoadSimpleExtension(t *testing.T) {
	tests := []struct {
		name        string
		sources     []testExtensionSource
		expectedLog string
	}{{
		name:        "empty",
		sources:     nil,
		expectedLog: "",
	}, {
		name: "no-init (single)",
		sources: []testExtensionSource{{
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
		sources: []testExtensionSource{{
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
		sources: []testExtensionSource{{
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
		sources: []testExtensionSource{{
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
		sources: []testExtensionSource{{
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
			loader := &testExtensionLoader{
				sources: map[string][]testExtensionSource{
					"test": test.sources,
				},
			}
			opts := &starform.ExtensionSetOptions{
				PrintHandler: printHandler,
				Loader:       loader,
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
