package starform_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
	"github.com/canonical/starlark/syntax"
)

type unsafeTestStringer struct {
	// Allows test errors to be declared in methods without error returns.
	t startest.TestBase
}

var _ starlark.Value = &unsafeTestStringer{}

func (uts *unsafeTestStringer) Freeze()               {}
func (uts *unsafeTestStringer) Truth() starlark.Bool  { return starlark.False }
func (uts *unsafeTestStringer) Type() string          { return "unsafeTestStringer" }
func (uts *unsafeTestStringer) Hash() (uint32, error) { return 0, nil }
func (uts *unsafeTestStringer) String() string {
	uts.t.Error("String called")
	return ""
}

type testSafeStringer struct {
	t          startest.TestBase
	safeString func(thread *starlark.Thread, sb starlark.StringBuilder) error
}

var _ starlark.Value = &testSafeStringer{}
var _ starlark.SafeStringer = &testSafeStringer{}

func (tss *testSafeStringer) Freeze()               {}
func (tss *testSafeStringer) Truth() starlark.Bool  { return starlark.False }
func (tss *testSafeStringer) Type() string          { return "testSafeStringer" }
func (tss *testSafeStringer) Hash() (uint32, error) { return 0, nil }
func (tss *testSafeStringer) String() string {
	tss.t.Error("String called")
	return ""
}
func (tss *testSafeStringer) SafeString(thread *starlark.Thread, sb starlark.StringBuilder) error {
	if tss.safeString == nil {
		return errors.New("testSafeStringer called with nil safeString function")
	}
	return tss.safeString(thread, sb)
}

func TestPrintSafety(t *testing.T) {
	testLogSafety(t, starform.PrintBuiltin)
}

func TestDebugSafety(t *testing.T) {
	testLogSafety(t, starform.DebugBuiltin)
}

func testLogSafety(t *testing.T, builtin *starlark.Builtin) {
	logger := starform.NewScriptLogger(&testLogger{}, "")
	builtin = builtin.BindReceiver(logger)

	safeties := []starlark.SafetyFlags{starlark.CPUSafe, starlark.MemSafe, starlark.TimeSafe, starlark.IOSafe}
	for _, safety := range safeties {
		t.Run(safety.String(), func(t *testing.T) {
			t.Run("argument", func(t *testing.T) {
				thread := &starlark.Thread{}
				thread.Print = func(thread *starlark.Thread, msg string) {
					t.Error("unexpected print call")
				}
				thread.RequireSafety(safety)
				starform.SetEventObject(thread, &starform.EventObject{})

				stringer := &unsafeTestStringer{t: t}
				_, err := starlark.Call(thread, builtin, starlark.Tuple{stringer}, nil)
				if err == nil {
					t.Error("expected error")
				} else if !errors.Is(err, starlark.ErrSafety) {
					t.Errorf("unexpected error: %v", err)
				}
			})

			t.Run("no-print", func(t *testing.T) {
				thread := &starlark.Thread{}
				thread.RequireSafety(safety)
				starform.SetEventObject(thread, &starform.EventObject{})

				_, err := starlark.Call(thread, builtin, starlark.Tuple{starlark.None}, nil)
				if err != nil {
					t.Error("unexpected error")
				}
			})
		})
	}
}

func TestPrintSteps(t *testing.T) {
	testLogSteps(t, starform.PrintBuiltin)
}

func TestDebugSteps(t *testing.T) {
	testLogSteps(t, starform.DebugBuiltin)
}

func testLogSteps(t *testing.T, builtin *starlark.Builtin) {
	logger := starform.NewScriptLogger(&testLogger{}, "")
	builtin = builtin.BindReceiver(logger)

	const arbitraryAddedSteps = 100

	callFunction := func(value starlark.Value, args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
		result, _ := starlark.Call(&starlark.Thread{}, value, args, kwargs)
		return result
	}
	callMethod := func(value starlark.Value, method string, args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
		attr, _ := value.(starlark.HasAttrs).Attr(method)
		return callFunction(attr, args, kwargs)
	}

	tests := []struct {
		name  string
		input starlark.Value
		steps uint64
	}{{
		name:  "Bool",
		input: starlark.True,
		steps: uint64(len("True")),
	}, {
		name: "Builtin",
		input: starlark.NewBuiltin("foo", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, nil
		}),
		steps: uint64(len("<built-in function foo>")),
	}, {
		name: "Dict",
		input: func() *starlark.Dict {
			dict := starlark.NewDict(2)
			dict.SetKey(starlark.MakeInt(1), starlark.None)
			dict.SetKey(starlark.MakeInt(2), &testSafeStringer{
				safeString: func(thread *starlark.Thread, sb starlark.StringBuilder) error {
					// Writes nothing
					return thread.AddSteps(arbitraryAddedSteps)
				},
			})
			return dict
		}(),
		steps: uint64(len("{1: None, 2: }")) + arbitraryAddedSteps + 2, // +2 for values traversed.
	}, {
		name:  "Float",
		input: starlark.Float(3.14),
		steps: uint64(len("3.14")),
	}, {
		name: "Function",
		input: func() *starlark.Function {
			const filename = "test.star"
			const expr = "True"
			fn, err := starlark.ExprFuncOptions(&syntax.FileOptions{}, filename, expr, nil)
			if err != nil {
				t.Fatal(err)
			}
			return fn
		}(),
		steps: uint64(len("<function <expr>>")),
	}, {
		name:  "Int(small)",
		input: starlark.MakeInt(10),
		steps: uint64(len("10")),
	}, {
		name:  "Int(big)",
		input: starlark.MakeInt64(1 << 32),
		steps: uint64(len(fmt.Sprintf("%d", int64(1<<32)))),
	}, {
		name: "List",
		input: starlark.NewList([]starlark.Value{
			starlark.None,
			&testSafeStringer{
				safeString: func(thread *starlark.Thread, sb starlark.StringBuilder) error {
					// Writes nothing
					return thread.AddSteps(arbitraryAddedSteps)
				},
			},
		}),
		steps: uint64(len("[None, ]")) + arbitraryAddedSteps + 2,
	}, {
		name:  "None",
		input: starlark.None,
		steps: uint64(len("None")),
	}, {
		name: "Set",
		input: func() *starlark.Set {
			set := starlark.NewSet(2)
			set.Insert(starlark.None)
			set.Insert(&testSafeStringer{
				safeString: func(thread *starlark.Thread, sb starlark.StringBuilder) error {
					// Writes nothing
					return thread.AddSteps(arbitraryAddedSteps)
				},
			})
			return set
		}(),
		steps: uint64(len("set([None, ])")) + arbitraryAddedSteps + 2,
	}, {
		name: "Tuple",
		input: starlark.Tuple{
			starlark.None,
			&testSafeStringer{
				safeString: func(thread *starlark.Thread, _ starlark.StringBuilder) error {
					// Writes nothing
					return thread.AddSteps(100)
				},
			},
		},
		steps: uint64(len("(None, )")) + 2 + 100,
	}, {
		name:  "Bytes elems",
		input: callMethod(starlark.Bytes("test"), "elems", nil, nil),
		steps: uint64(len(`b"test".elems()`)),
	}, {
		name:  "Range",
		input: callFunction(starlark.Universe["range"], starlark.Tuple{starlark.MakeInt(0), starlark.MakeInt(10), starlark.MakeInt(2)}, nil),
		steps: uint64(len("range(0, 10, 2)")),
	}, {
		name:  "String elems (chars)",
		input: callMethod(starlark.String("test"), "elems", nil, nil),
		steps: uint64(len(`"test".elems()`)),
	}, {
		name:  "String elems (ords)",
		input: callMethod(starlark.String("test"), "elem_ords", nil, nil),
		steps: uint64(len(`"test".elem_ords()`)),
	}, {
		name:  "String codepoints (chars)",
		input: callMethod(starlark.String("test"), "codepoints", nil, nil),
		steps: uint64(len(`"test".codepoints()`)),
	}, {
		name:  "String codepoints (ords)",
		input: callMethod(starlark.String("test"), "codepoint_ords", nil, nil),
		steps: uint64(len(`"test".codepoint_ords()`)),
	}, {
		name:  "String",
		input: starlark.String("test"),
		steps: uint64(len("test")),
	}, {
		name:  "Bytes",
		input: starlark.Bytes("test"),
		steps: uint64(len(`test`)),
	}, {
		name:  "Bytes (invalid utf8)",
		input: starlark.Bytes(string([]byte{0x80, 0x80, 0x80, 0x80})),
		steps: uint64(len([]byte{0x80, 0x80, 0x80, 0x80})),
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := startest.From(t)
			st.RequireSafety(starlark.CPUSafe)
			st.SetMinSteps(test.steps)
			st.SetMaxSteps(test.steps)
			st.RunThread(func(thread *starlark.Thread) {
				thread.Print = func(thread *starlark.Thread, msg string) {
					t.Error("unexpected print call")
				}
				starform.SetEventObject(thread, &starform.EventObject{})
				for i := 0; i < st.N; i++ {
					_, err := starlark.Call(thread, builtin, starlark.Tuple{test.input}, nil)
					if err != nil {
						st.Error(err)
					}
				}
			})
		})
	}
}

func TestPrintAllocs(t *testing.T) {
	testLogAllocs(t, starform.PrintBuiltin)
}

func TestDebugAllocs(t *testing.T) {
	testLogAllocs(t, starform.DebugBuiltin)
}

func testLogAllocs(t *testing.T, builtin *starlark.Builtin) {
	listWithLoop := starlark.NewList(nil)
	listWithLoop.Append(listWithLoop)

	dictWithLoop := starlark.NewDict(1)
	dictWithLoop.SetKey(starlark.MakeInt(0x1CEB00DA), dictWithLoop)

	args := starlark.Tuple{
		starlark.True,
		listWithLoop,
		dictWithLoop,
		starlark.Float(math.Phi),
		starlark.NewSet(1),
		starlark.String(`"'{}🌋`),
	}

	st := startest.From(t)
	st.RequireSafety(starlark.MemSafe)
	st.RunThread(func(thread *starlark.Thread) {
		logger := starform.NewScriptLogger(&testLogger{}, "")
		builtin = builtin.BindReceiver(logger)
		thread.Print = func(thread *starlark.Thread, msg string) {
			t.Error("unexpected print call")
		}
		starform.SetEventObject(thread, &starform.EventObject{})

		for i := 0; i < st.N; i++ {
			res, err := starlark.Call(thread, builtin, args, nil)
			if err != nil {
				st.Error(err)
			}
			st.KeepAlive(res)
		}
		st.KeepAlive(logger)
	})
}

func TestPrintCancellation(t *testing.T) {
	testLogCancellation(t, starform.PrintBuiltin)
}

func TestDebugCancellation(t *testing.T) {
	testLogCancellation(t, starform.DebugBuiltin)
}

func testLogCancellation(t *testing.T, builtin *starlark.Builtin) {
	logger := starform.NewScriptLogger(&testLogger{}, "")
	builtin = builtin.BindReceiver(logger)

	callFunction := func(value starlark.Value, args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
		result, _ := starlark.Call(&starlark.Thread{}, value, args, kwargs)
		return result
	}
	callMethod := func(value starlark.Value, method string, args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
		attr, _ := value.(starlark.HasAttrs).Attr(method)
		return callFunction(attr, args, kwargs)
	}

	tests := []struct {
		name  string
		input func(n int) starlark.Value
	}{{
		name: "Bytes",
		input: func(n int) starlark.Value {
			return starlark.Bytes(strings.Repeat("a", n))
		},
	}, {
		name: "Bytes (invalid utf8)",
		input: func(n int) starlark.Value {
			return starlark.Bytes(strings.Repeat(string([]byte{0x80}), n))
		},
	}, {
		name: "Dict",
		input: func(n int) starlark.Value {
			dict := starlark.NewDict(n)
			for i := 0; i < n; i++ {
				// Int hash only uses the least 32 bits.
				// Leaving them blank creates collisions.
				key := starlark.MakeInt64(int64(i) << 32)
				dict.SetKey(key, starlark.None)
			}
			return dict
		},
	}, {
		name: "Int(big)",
		input: func(n int) starlark.Value {
			return starlark.MakeInt64(1 << n)
		},
	}, {
		name: "List",
		input: func(n int) starlark.Value {
			elems := make([]starlark.Value, n)
			for i := range elems {
				elems[i] = starlark.None
			}
			return starlark.NewList(elems)
		},
	}, {
		name: "Set",
		input: func(n int) starlark.Value {
			set := starlark.NewSet(n)
			for i := 0; i < n; i++ {
				// Int hash only uses the least 32 bits.
				// Leaving them blank creates collisions.
				key := starlark.MakeInt64(int64(i) << 32)
				set.Insert(key)
			}
			return set
		},
	}, {
		name: "Tuple",
		input: func(n int) starlark.Value {
			elems := make([]starlark.Value, n)
			for i := range elems {
				elems[i] = starlark.None
			}
			return starlark.Tuple(elems)
		},
	}, {
		name: "Bytes elems",
		input: func(n int) starlark.Value {
			return callMethod(starlark.Bytes(strings.Repeat("a", n)), "elems", nil, nil)
		},
	}, {
		name: "Range",
		input: func(n int) starlark.Value {
			return callFunction(starlark.Universe["range"], starlark.Tuple{starlark.MakeInt(0), starlark.MakeInt(n)}, nil)
		},
	}, {
		name: "String elems (chars)",
		input: func(n int) starlark.Value {
			return callMethod(starlark.String(strings.Repeat("a", n)), "elems", nil, nil)
		},
	}, {
		name: "String elems (ords)",
		input: func(n int) starlark.Value {
			return callMethod(starlark.String(strings.Repeat("a", n)), "elem_ords", nil, nil)
		},
	}, {
		name: "String codepoints (chars)",
		input: func(n int) starlark.Value {
			return callMethod(starlark.String(strings.Repeat("a", n)), "codepoints", nil, nil)
		},
	}, {
		name: "String codepoints (ords)",
		input: func(n int) starlark.Value {
			return callMethod(starlark.String(strings.Repeat("a", n)), "codepoint_ords", nil, nil)
		},
	}, {
		name: "SafeStringer",
		input: func(n int) starlark.Value {
			return &testSafeStringer{
				safeString: func(thread *starlark.Thread, sb starlark.StringBuilder) error {
					for i := 0; i < n; i++ {
						if _, err := sb.WriteString("foo"); err != nil {
							return err
						}
					}
					return nil
				},
			}
		},
	}, {
		name: "String",
		input: func(n int) starlark.Value {
			return starlark.String(strings.Repeat("a", n))
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := startest.From(t)
			st.RequireSafety(starlark.TimeSafe)
			st.SetMaxSteps(0)
			st.RunThread(func(thread *starlark.Thread) {
				thread.Cancel("done")
				thread.Print = func(thread *starlark.Thread, msg string) {
					t.Error("unexpected print call")
				}
				starform.SetEventObject(thread, &starform.EventObject{})
				_, err := starlark.Call(thread, builtin, starlark.Tuple{test.input(st.N)}, nil)
				if err == nil {
					st.Error("expected cancellation")
				} else if !isStarlarkCancellation(err) {
					st.Errorf("expected cancellation, got: %v", err)
				}
			})
		})
	}
}
