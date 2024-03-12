package starform_test

import (
	"context"
	"errors"
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

func TestLoadStatement(t *testing.T) {
	tests := []struct {
		name        string
		sources     []starform.ScriptSource
		expectedLog string
	}{{
		name: "sorted",
		sources: []starform.ScriptSource{&testScriptSource{
			name: "1.star",
			content: `
				print("first")
				third_str = "third"

				def init():
					print("fourth")
			`,
		}, &testScriptSource{
			name: "2.star",
			content: `
				print("second")
				load("1.star", "third_str")
				print(third_str)

				def init():
					print("fifth")
			`,
		}},
		expectedLog: "first\nsecond\nthird\nfourth\nfifth\n",
	}, {
		name: "unsorted",
		sources: []starform.ScriptSource{&testScriptSource{
			name: "2.star",
			content: `
				print("second")
				third_str = "third"

				def init():
					print("sixth")
			`,
		}, &testScriptSource{
			name: "1.star",
			content: `
				print("first")
				load("2.star", "third_str")
				print(third_str)

				def init():
					print("fifth")
			`,
		}, &testScriptSource{
			name: "3.star",
			content: `
				print("fourth")

				def init():
					print("seventh")
			`,
		}},
		expectedLog: "first\nsecond\nthird\nfourth\nfifth\nsixth\nseventh\n",
	}, {
		name: "multiple loads",
		sources: []starform.ScriptSource{&testScriptSource{
			name: "1.star",
			content: `
				print("first")
				load("2.star", "third_str")
				print(third_str)

				def init():
					print("sixth")
			`,
		}, &testScriptSource{
			name: "2.star",
			content: `
				print("second")
				third_str = "third"
				fifth_str = "fifth"

				def init():
					print("seventh")
			`,
		}, &testScriptSource{
			name: "3.star",
			content: `
				print("fourth")
				load("2.star", "fifth_str")
				print(fifth_str)

				def init():
					print("eighth")
			`,
		}},
		expectedLog: "first\nsecond\nthird\nfourth\nfifth\nsixth\nseventh\neighth\n",
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
			if err := scripts.LoadSources(context.Background(), test.sources); err != nil {
				t.Error(err)
			}

			if actualLog := log.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v got %v", test.expectedLog, actualLog)
			}
		})
	}

	t.Run("nonexistent", func(t *testing.T) {
		opts := &starform.ScriptSetOptions{
			PrintHandler: func(thread *starlark.Thread, msg string) {
				t.Errorf("unexpected print call: %s", msg)
			},
		}
		sources := []starform.ScriptSource{&testScriptSource{
			name: "test.star",
			content: `
				load("nonexistent.star", "foo")
				fail("unexpectedly called")
			`,
		}}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := scripts.LoadSources(context.Background(), sources); err == nil {
			t.Fatal("expected error, got success")
		}
	})

	t.Run("load-cycle", func(t *testing.T) {
		opts := &starform.ScriptSetOptions{
			PrintHandler: func(thread *starlark.Thread, msg string) {
				t.Errorf("unexpected print call: %s", msg)
			},
		}
		sources := []starform.ScriptSource{&testScriptSource{
			name: "chicken.star",
			content: `
				load("egg.star", "x")
				fail("egg came first")
			`,
		}, &testScriptSource{
			name: "egg.star",
			content: `
				load("chicken.star", "x")
				fail("chicken came first")
			`,
		}}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := scripts.LoadSources(context.Background(), sources); err == nil {
			t.Fatalf("expected error, got success")
		} else {
			t.Log(err)
		}
	})
}
