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

type testLogger struct {
	strings.Builder
}

func (tl *testLogger) PrintHandler() func(thread *starlark.Thread, msg string) {
	return func(thread *starlark.Thread, msg string) {
		tl.WriteString(msg)
		tl.WriteByte('\n')
	}
}

func TestRunSimpleScriptlet(t *testing.T) {
	log := &testLogger{}
	opts := &starform.ExtensionSetOptions{
		PrintHandler: log.PrintHandler(),
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

var stdoutPrintHandler = func(thread *starlark.Thread, msg string) {
	fmt.Println(msg)
}

func TestCheckLoadPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect string
	}{{
		name: "simple",
		path: "aaa.star",
	}, {
		name: "complex",
		path: "aaa/bbb/ccc.star",
	}, {
		// 	name: "relative-simple",
		// 	path: "./aaa.star",
		// }, {
		name: "relative-complex",
		path: "./aaa/bbb/ccc.star",
	}, {
		name: "parent-simple",
		path: "../aaa.star",
	}, {
		name: "parent-complex",
		path: "../../../aaa/bbb/ccc.star",
	}, {
		name:   "empty",
		path:   "",
		expect: "cannot load '': path is empty",
	}, {
		name:   "absolute",
		path:   "/aaa.star",
		expect: "cannot load '/aaa.star': path is absolute",
	}, {
		name:   "wrong-extension",
		path:   "aaa.png",
		expect: "cannot load 'aaa.png': path must have '.star' extension",
	}, {
		name:   "short-components",
		path:   "aa/bb/ccc.star",
		expect: "cannot load 'a/b/ccc.star': ",
	}, {
		name:   "short-stem",
		path:   "aaa/bbb/cc.star",
		expect: "cannot load 'aaa/bbb/cc.star': ",
	}, {
		name:   "invalid-rune-dash",
		path:   "---.star",
		expect: "cannot load '': ",
	}, {
		name:   "invalid-rune-emoji",
		path:   "🤸🪑🏌️.star",
		expect: "cannot load '': ",
	}, {
		name:   "hidden-files",
		path:   ".secret.star",
		expect: "cannot load '': ",
	}, {
		name:   "midway-dots",
		path:   "aaa/b.b/ccc.star",
		expect: "cannot load '': ",
	}, {
		name:   "successive-dots",
		path:   "aaa/.../bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "midway-current-dir",
		path:   "aaa/../bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "midway-parent-dir",
		path:   "aaa/../bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "successive-slashes",
		path:   "aaa//bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "successive-underscores",
		path:   "a__a.star",
		expect: "cannot load '': ",
	}, {
		name:   "leading-underscore",
		path:   "aaa/_bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "trailing-underscore",
		path:   "aaa_/bbb.star",
		expect: "cannot load '': ",
	}, {
		name:   "trailing-underscore-before-extension",
		path:   "aaa/bbb_.star",
		expect: "cannot load '': ",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := &starform.ExtensionSetOptions{
				PrintHandler: stdoutPrintHandler,
				Loader: &testSourceLoader{
					sources: map[string][]testSource{
						"test": {{
							name:    "init.star",
							content: fmt.Sprintf("load('%s', 'unused')", test.path),
							hash:    123,
						}, {
							name:    test.path,
							content: "unused = None",
							hash:    456,
						}},
					},
				},
			}
			set, err := starform.NewExtensionSet(opts)
			if err != nil {
				t.Fatal(err)
			}

			_, err = set.Load("test")
			if err != nil {
				if test.expect == "" {
					t.Errorf("unexpected error: %q", err)
				} else if err.Error() != test.expect {
					t.Errorf("unexpected error: expected %q but got %q", test.expect, err)
				}
			} else if test.expect != "" {
				t.Errorf("expected error")
			}
		})
	}
}
