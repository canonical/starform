package starform_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type testScriptSource struct {
	name, content string
}

var _ starform.ScriptSource = &testScriptSource{}

func (tss *testScriptSource) Path() string { return tss.name }
func (tss *testScriptSource) Content(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	content, err := startest.Reindent(tss.content)
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
		name: "NotSafe",
		opts: &starform.ScriptSetOptions{},
	}, {
		name: "MemSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			RequiredSafety: starlark.MemSafe,
		},
		expected: "cannot run MemSafe Starlark with unbounded MaxAllocs",
	}, {
		name: "MemSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			RequiredSafety: starlark.MemSafe,
			MaxAllocs:      100,
		},
	}, {
		name: "CPUSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			RequiredSafety: starlark.CPUSafe,
		},
		expected: "cannot run CPUSafe Starlark with unbounded MaxSteps",
	}, {
		name: "CPUSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			RequiredSafety: starlark.CPUSafe,
			MaxSteps:       100,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := starform.NewScriptSet(test.opts)
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
		sources     []starform.ScriptSource
		expectedLog string
	}{{
		name:        "empty",
		sources:     nil,
		expectedLog: "",
	}, {
		name: "no-init (single)",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "single.star",
				content: `
					print("single")
					def do_not_call():
						fail('unexpectedly called')
				`,
			},
		},
		expectedLog: "single\n",
	}, {
		name: "no-init (multiple)",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "2.star",
				content: `
					print("second")
					def do_not_call():
						fail('unexpectedly called')
				`,
			}, &testScriptSource{
				name: "1.star",
				content: `
					print("first")
					def do_not_call():
						fail('unexpectedly called')
				`,
			},
		},
		expectedLog: "first\nsecond\n",
	}, {
		name: "init (single)",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "single.star",
				content: `
					print("Hello,")
					def init():
						print("world!")
				`,
			},
		},
		expectedLog: "Hello,\nworld!\n",
	}, {
		name: "init (multiple)",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "lib.star",
				content: `
					print("second")
					def do_not_call():
						fail('unexpectedly called')
				`,
			}, &testScriptSource{
				name: "init.star",
				content: `
					def init():
						print("third")
					print("first")
				`,
			},
		},
		expectedLog: "first\nsecond\nthird\n",
	}, {
		name: "unsupported extensions",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "index.html",
				content: `
					<b>not a script</b>
				`,
			}, &testScriptSource{
				name: "lib.star.swp",
				content: `
					print("vim cache file")
				`,
			}, &testScriptSource{
				name: "README.md",
				content: `
					# Docs
				`,
			},
		},
		expectedLog: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := &strings.Builder{}
			printHandler := func(thread *starlark.Thread, msg string) {
				log.WriteString(msg)
				log.WriteByte('\n')
			}
			opts := &starform.ScriptSetOptions{
				PrintHandler: printHandler,
			}
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			if scripts == nil {
				t.Fatalf("returned set should not be nil")
			}

			if err := scripts.LoadSources(context.Background(), test.sources); err != nil {
				t.Error(err)
			}
			if actualLog := log.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v go %v", test.expectedLog, actualLog)
			}
		})
	}
}

func TestCheckLoadPath(t *testing.T) {
	miscError := func(path string) string {
		return fmt.Sprintf("cannot load %q: path invalid, see https://github.com/canonical/starlark/blob/main/doc/valid-load-paths.md", path)
	}

	tests := []struct {
		name   string
		path   string
		expect string
	}{{
		name: "toplevel",
		path: "abcdefghijklmnopqrstuvwxyz_0123456789.star",
	}, {
		name: "nested",
		path: "aaa/bbb/ccc.star",
	}, {
		name: "relative-toplevel",
		path: "./aaa.star",
	}, {
		name: "parent-nested",
		path: "../aaa.star",
	}, {
		name: "parent-nested",
		path: "../../../aaa/bbb/ccc.star",
	}, {
		name:   "invalid-rune-dash",
		path:   "---.star",
		expect: `cannot load "---.star": path contains "-", use "_" instead`,
	}, {
		name:   "backslashes",
		path:   `.\\.\\aaa.star`,
		expect: `cannot load ".\.\aaa.star": path contains "\", use "/" instead`,
	}, {
		name:   "extra-starting-current-dir",
		path:   "././aaa.star",
		expect: `cannot load "././aaa.star": path contains redundant components`,
	}, {
		name:   "current-dir-in-parent-dir",
		path:   ".././aaa.star",
		expect: `cannot load ".././aaa.star": path contains redundant components`,
	}, {
		name:   "parent-op-in-current-dir",
		path:   "./../aaa.star",
		expect: `cannot load "./../aaa.star": path contains redundant components`,
	}, {
		name:   "midway-current-dir",
		path:   "aaa/./bbb.star",
		expect: `cannot load "aaa/./bbb.star": path contains redundant components`,
	}, {
		name:   "midway-parent-dir",
		path:   "aaa/../bbb.star",
		expect: `cannot load "aaa/../bbb.star": path contains redundant components`,
	}, {
		name:   "successive-slashes",
		path:   "aaa//bbb.star",
		expect: `cannot load "aaa//bbb.star": path contains redundant components`,
	}, {
		name:   "empty",
		path:   "",
		expect: miscError(""),
	}, {
		name:   "absolute",
		path:   "/aaa.star",
		expect: miscError("/aaa.star"),
	}, {
		name:   "wrong-extension",
		path:   "aaa.png",
		expect: miscError("aaa.png"),
	}, {
		name:   "short-components",
		path:   "aa/bb/ccc.star",
		expect: miscError("aa/bb/ccc.star"),
	}, {
		name:   "short-stem",
		path:   "aa.star",
		expect: miscError("aa.star"),
	}, {
		name:   "nested-short-stem",
		path:   "aaa/bbb/cc.star",
		expect: miscError("aaa/bbb/cc.star"),
	}, {
		name:   "uppercase-forbidden",
		path:   "AAA.star",
		expect: miscError("AAA.star"),
	}, {
		name:   "invalid-rune-emoji",
		path:   "🤸🪑🏌️.star",
		expect: miscError("🤸🪑🏌️.star"),
	}, {
		name:   "no-stem",
		path:   ".star",
		expect: miscError(".star"),
	}, {
		name:   "hidden-files",
		path:   ".secret.star",
		expect: miscError(".secret.star"),
	}, {
		name:   "hidden-dirs",
		path:   "aaa/.secret/bbb.star",
		expect: miscError("aaa/.secret/bbb.star"),
	}, {
		name:   "midway-dots",
		path:   "aaa/b.b/ccc.star",
		expect: miscError("aaa/b.b/ccc.star"),
	}, {
		name:   "many-extensions",
		path:   "aaa/bbb.tar.star",
		expect: miscError("aaa/bbb.tar.star"),
	}, {
		name:   "successive-dots-as-component",
		path:   "aaa/.../bbb.star",
		expect: miscError("aaa/.../bbb.star"),
	}, {
		name:   "successive-dots-in-component",
		path:   "aaa..bbb.star",
		expect: miscError("aaa..bbb.star"),
	}, {
		name:   "successive-underscores",
		path:   "a__a.star",
		expect: miscError("a__a.star"),
	}, {
		name:   "leading-underscore",
		path:   "_aaa.star",
		expect: miscError("_aaa.star"),
	}, {
		name:   "midway-leading-underscore",
		path:   "aaa/_bbb.star",
		expect: miscError("aaa/_bbb.star"),
	}, {
		name:   "trailing-underscore",
		path:   "aaa_/bbb.star",
		expect: miscError("aaa_/bbb.star"),
	}, {
		name:   "trailing-underscore-before-extension",
		path:   "aaa/bbb_.star",
		expect: miscError("aaa/bbb_.star"),
	}, {
		name:   "underscore-before-dot",
		path:   "aaa/b_.b.star",
		expect: miscError("aaa/b_.b.star"),
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := &starform.ScriptSetOptions{}
			set, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}

			sources := []starform.ScriptSource{
				&testScriptSource{
					name:    "init.star",
					content: fmt.Sprintf("load('%s', 'unused')", test.path),
				}, &testScriptSource{
					name:    test.path,
					content: "unused = None",
				},
			}
			err = set.LoadSources(context.Background(), sources)
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

func TestCancelLoad(t *testing.T) {
	opts := &starform.ScriptSetOptions{}
	scripts, err := starform.NewScriptSet(opts)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = scripts.LoadSources(ctx, []starform.ScriptSource{&testScriptSource{
		name: "test.star",
		content: `
			def init():
				fail('unexpectedly called')
		`,
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: expected %v got %v", context.Canceled, err)
	}
}
