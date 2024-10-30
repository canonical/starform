package starform_test

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/startest"
)

type testLogger struct {
	log    strings.Builder
	format func(starform.LogEntry) string
}

var _ starform.Logger = &testLogger{}

func (tl *testLogger) Log(ctx context.Context, entry starform.LogEntry) {
	if tl.format == nil {
		tl.format = func(le starform.LogEntry) string {
			return le.Message + "\n"
		}
	}
	message := tl.format(entry)
	tl.log.WriteString(message)
}

func (tl *testLogger) String() string { return tl.log.String() }

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

type testScriptCache struct {
	starform.TestCacheBase
	mu     sync.RWMutex
	cache  map[interface{}]interface{}
	Misses int32
}

var _ starform.ScriptCache = &testScriptCache{}

func (tsc *testScriptCache) Get(key interface{}) (interface{}, error) {
	tsc.mu.RLock()
	defer tsc.mu.RUnlock()

	if program, ok := tsc.cache[key]; ok {
		return program, nil
	}
	atomic.AddInt32(&tsc.Misses, 1)
	return nil, starform.ErrNotCached
}

func (tsc *testScriptCache) Put(key, value interface{}, source starform.ScriptSource) error {
	tsc.mu.Lock()
	defer tsc.mu.Unlock()

	if tsc.cache == nil {
		tsc.cache = make(map[interface{}]interface{})
	}
	if _, ok := tsc.cache[key]; !ok {
		tsc.cache[key] = value
	}
	return nil
}

func (tsc *testScriptCache) Drop(key interface{}) {
	tsc.mu.Lock()
	defer tsc.mu.Unlock()

	delete(tsc.cache, key)
}

func (tsc *testScriptCache) Len() int {
	tsc.mu.RLock()
	defer tsc.mu.RUnlock()

	return len(tsc.cache)
}

func (tsc *testScriptCache) Visit(f func(key, value interface{}) error) error {
	tsc.mu.RLock()
	defer tsc.mu.RUnlock()

	for k, v := range tsc.cache {
		tsc.mu.RUnlock()
		err := f(k, v)
		tsc.mu.RLock()
		if err != nil {
			return err
		}
	}

	return nil
}

func TestOptionsValidation(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}
	tests := []struct {
		name     string
		opts     *starform.ScriptSetOptions
		expected string
	}{{
		name: "NotSafe",
		opts: &starform.ScriptSetOptions{
			App: app,
		},
	}, {
		name: "MemSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			App:            app,
			RequiredSafety: starlark.MemSafe,
		},
		expected: "cannot create script set: MemSafe requested but no MaxAllocs set",
	}, {
		name: "MemSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			App:            app,
			RequiredSafety: starlark.MemSafe,
			MaxAllocs:      100,
		},
	}, {
		name: "CPUSafe (unbounded)",
		opts: &starform.ScriptSetOptions{
			App:            app,
			RequiredSafety: starlark.CPUSafe,
		},
		expected: "cannot create script set: CPUSafe requested but no MaxSteps set",
	}, {
		name: "CPUSafe (bounded)",
		opts: &starform.ScriptSetOptions{
			App:            app,
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
	app := &starform.AppObject{
		Name: "test",
	}
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
				name: "222.star",
				content: `
					print("second")
					def do_not_call():
						fail('unexpectedly called')
				`,
			}, &testScriptSource{
				name: "111.star",
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
			logger := &testLogger{}
			opts := &starform.ScriptSetOptions{
				App:    app,
				Logger: logger,
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
			if actualLog := logger.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v go %v", test.expectedLog, actualLog)
			}
		})
	}
}

func TestCheckLoadPath(t *testing.T) {
	miscError := func(path string) string {
		return fmt.Sprintf("cannot load %s: path invalid, see https://github.com/canonical/starlark/blob/main/doc/valid-load-paths.md", path)
	}

	app := &starform.AppObject{
		Name: "test",
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
		expect: `cannot load ---.star: path contains "-", use "_" instead`,
	}, {
		name:   "backslashes",
		path:   `.\\.\\aaa.star`,
		expect: `cannot load .\\.\\aaa.star: path contains "\", use "/" instead`,
	}, {
		name:   "extra-starting-current-dir",
		path:   "././aaa.star",
		expect: `cannot load ././aaa.star: path contains redundant components`,
	}, {
		name:   "current-dir-in-parent-dir",
		path:   ".././aaa.star",
		expect: `cannot load .././aaa.star: path contains redundant components`,
	}, {
		name:   "parent-op-in-current-dir",
		path:   "./../aaa.star",
		expect: `cannot load ./../aaa.star: path contains redundant components`,
	}, {
		name:   "midway-current-dir",
		path:   "aaa/./bbb.star",
		expect: `cannot load aaa/./bbb.star: path contains redundant components`,
	}, {
		name:   "midway-parent-dir",
		path:   "aaa/../bbb.star",
		expect: `cannot load aaa/../bbb.star: path contains redundant components`,
	}, {
		name:   "successive-slashes",
		path:   "aaa//bbb.star",
		expect: `cannot load aaa//bbb.star: path contains redundant components`,
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
		path:   "aaa.starlark",
		expect: miscError("aaa.starlark"),
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
			opts := &starform.ScriptSetOptions{
				App: app,
			}
			set, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}

			sanitisedPath := path.Clean(test.path)
			for strings.HasPrefix(sanitisedPath, "./") {
				sanitisedPath = sanitisedPath[2:]
			}
			numTestPathParents := 0
			for strings.HasPrefix(sanitisedPath, "../") {
				sanitisedPath = sanitisedPath[3:]
				numTestPathParents++
			}
			if strings.Count(sanitisedPath, "../") > 1 {
				t.Fatal("more than one occurrence of '../' after initial path operators is not supported")
			}
			sources := []starform.ScriptSource{
				&testScriptSource{
					name:    strings.Repeat("dir/", numTestPathParents) + "init.star",
					content: fmt.Sprintf("load('%s', 'unused')", test.path),
				}, &testScriptSource{
					name:    sanitisedPath,
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
	app := &starform.AppObject{
		Name: "test",
	}
	opts := &starform.ScriptSetOptions{
		App: app,
	}
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
	app := &starform.AppObject{
		Name: "test",
	}
	tests := []struct {
		name        string
		sources     []starform.ScriptSource
		expectedLog string
	}{{
		name: "sorted",
		sources: []starform.ScriptSource{&testScriptSource{
			name: "111.star",
			content: `
				print("first")
				third_str = "third"

				def init():
					print("fourth")
			`,
		}, &testScriptSource{
			name: "222.star",
			content: `
				print("second")
				load("111.star", "third_str")
				print(third_str)

				def init():
					print("fifth")
			`,
		}},
		expectedLog: "first\nsecond\nthird\nfourth\nfifth\n",
	}, {
		name: "unsorted",
		sources: []starform.ScriptSource{&testScriptSource{
			name: "222.star",
			content: `
				print("second")
				third_str = "third"

				def init():
					print("sixth")
			`,
		}, &testScriptSource{
			name: "111.star",
			content: `
				print("first")
				load("222.star", "third_str")
				print(third_str)

				def init():
					print("fifth")
			`,
		}, &testScriptSource{
			name: "333.star",
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
			name: "111.star",
			content: `
				print("first")
				load("222.star", "third_str")
				print(third_str)

				def init():
					print("sixth")
			`,
		}, &testScriptSource{
			name: "222.star",
			content: `
				print("second")
				third_str = "third"
				fifth_str = "fifth"

				def init():
					print("seventh")
			`,
		}, &testScriptSource{
			name: "333.star",
			content: `
				print("fourth")
				load("222.star", "fifth_str")
				print(fifth_str)

				def init():
					print("eighth")
			`,
		}},
		expectedLog: "first\nsecond\nthird\nfourth\nfifth\nsixth\nseventh\neighth\n",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := &testLogger{}
			opts := &starform.ScriptSetOptions{
				App:    app,
				Logger: logger,
			}
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := scripts.LoadSources(context.Background(), test.sources); err != nil {
				t.Error(err)
			}

			if actualLog := logger.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v got %v", test.expectedLog, actualLog)
			}
		})
	}

	t.Run("nonexistent loads", func(t *testing.T) {
		logger := &testLogger{}
		opts := &starform.ScriptSetOptions{
			App:    app,
			Logger: logger,
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
		if log := logger.String(); log != "" {
			t.Errorf("unexpected log output: %s", log)
		}
	})

	t.Run("load-cycle", func(t *testing.T) {
		logger := &testLogger{}
		opts := &starform.ScriptSetOptions{
			App:    app,
			Logger: logger,
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
		}
		if log := logger.String(); log != "" {
			t.Errorf("unexpected log output: %s", log)
		}
	})
}

func TestRelativeLoads(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}

	tests := []struct {
		name          string
		sources       []starform.ScriptSource
		expectedError string
		expectedLog   string
	}{{
		name: "absolute-toplevel",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa.star",
				content: `
					load('bbb.star', 'bbb')
					print('aaa.star:', bbb)
				`,
			}, &testScriptSource{
				name: "bbb.star",
				content: `
					bbb = 'bbb.star'
					print('bbb.star:')
				`,
			},
		},
		expectedLog: "bbb.star:\naaa.star: bbb.star\n",
	}, {
		name: "absolute-nested",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa/bbb.star",
				content: `
					load('ccc.star', 'ccc')
					print('aaa/bbb.star:', ccc)
				`,
			},
			&testScriptSource{
				name: "ccc.star",
				content: `
					ccc = 'ccc.star'
					print('ccc.star:')
				`,
			},
		},
		expectedLog: "ccc.star:\naaa/bbb.star: ccc.star\n",
	}, {
		name: "relative-toplevel",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa.star",
				content: `
					load('./bbb.star', 'bbb')
					print('aaa.star:', bbb)
				`,
			}, &testScriptSource{
				name: "bbb.star",
				content: `
					bbb = 'bbb.star'
					print('bbb.star:')
				`,
			},
		},
		expectedLog: "bbb.star:\naaa.star: bbb.star\n",
	}, {
		name: "relative-nested",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa/bbb.star",
				content: `
					load('aaa/ccc/ddd.star', 'ddd')
					print('aaa/bbb.star:', ddd)
				`,
			}, &testScriptSource{
				name: "aaa/ccc/ddd.star",
				content: `
					ddd = 'aaa/ccc/ddd.star'
					print('aaa/ccc/ddd.star:')
				`,
			},
		},
		expectedLog: "aaa/ccc/ddd.star:\naaa/bbb.star: aaa/ccc/ddd.star\n",
	}, {
		name: "parent-nested",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa/bbb/ccc.star",
				content: `
					load('../ddd/eee.star', 'eee')
					print('aaa/bbb/ccc.star:', eee)
				`,
			}, &testScriptSource{
				name: "aaa/ddd/eee.star",
				content: `
					eee = 'aaa/ddd/eee.star'
					print('aaa/ddd/eee.star:')
				`,
			},
		},
		expectedLog: "aaa/ddd/eee.star:\naaa/bbb/ccc.star: aaa/ddd/eee.star\n",
	}, {
		name: "parent-toplevel",
		sources: []starform.ScriptSource{
			&testScriptSource{
				name: "aaa.star",
				content: `
					load('../nonexistent.star', 'nonexistent')
					fail('unreachable')
				`,
			},
		},
		expectedError: "cannot load ../nonexistent.star: file not found",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := &testScriptCache{}
			logger := &testLogger{}
			opts := &starform.ScriptSetOptions{
				App:    app,
				Cache:  cache,
				Logger: logger,
			}
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			err = scripts.LoadSources(context.Background(), test.sources)
			if err == nil {
				if test.expectedError != "" {
					t.Error("expected error")
				}
			} else if err.Error() != test.expectedError {
				t.Errorf("unexpected error: expected %s but got %v", test.expectedError, err)
			}
			if actualLog := logger.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v got %v", test.expectedLog, actualLog)
			}
		})
	}
}

func TestCachedRelativeLoads(t *testing.T) {
	commonFile := &testScriptSource{
		name: "aaa/bbb/bbb.star",
		content: `
			load('../../ccc.star', 'ccc')
			bbb = 'aaa/bbb/bbb.star'
			print('aaa/bbb/bbb.star:', ccc)
		`,
	}
	setASources := []starform.ScriptSource{
		commonFile,
		&testScriptSource{
			name: "ccc.star",
			content: `
				load('ddd.star', 'ddd')
				ccc = 'ccc.star'
				print('ccc.star:')
			`,
		}, &testScriptSource{
			name: "ddd.star",
			content: `
				ddd = 'ddd.star'
			`,
		},
	}
	setBSources := []starform.ScriptSource{
		&testScriptSource{
			name: "aaa/aaa.star",
			content: `
				load('./bbb/bbb.star', 'bbb')
				load('../ccc.star', 'ccc')
				print('aaa/aaa.star:', bbb, ccc)
			`,
		},
		commonFile,
		&testScriptSource{
			name: "ccc.star",
			content: `
				ccc = 'ccc.star'
				print('ccc.star:')
			`,
		},
	}

	getLoadLog := func(cache starform.ScriptCache, sources []starform.ScriptSource) (string, error) {
		logger := &testLogger{
			format: func(le starform.LogEntry) string {
				return fmt.Sprintf("[%s]: %s\n", le.Path, le.Message)
			},
		}
		opts := &starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "test",
			},
			Cache:  cache,
			Logger: logger,
		}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			return "", err
		}
		err = scripts.LoadSources(context.Background(), sources)
		if err != nil {
			return "", err
		}
		return logger.String(), nil
	}

	cache := &testScriptCache{}
	if _, err := getLoadLog(cache, setASources); err != nil {
		t.Fatal(err)
	}

	expectedLogB, err := startest.Reindent(`
		[ccc.star]: ccc.star:
		[aaa/bbb/bbb.star]: aaa/bbb/bbb.star: ccc.star
		[aaa/aaa.star]: aaa/aaa.star: aaa/bbb/bbb.star ccc.star`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if actualLogB, err := getLoadLog(cache, setBSources); err != nil {
		t.Error(err)
	} else if actualLogB != expectedLogB {
		t.Errorf("incorrect log: expected %q but got %q", expectedLogB, actualLogB)
	}

	if expectedMisses := len(setASources) + len(setBSources) - 1; int(cache.Misses) != expectedMisses {
		t.Errorf("caching failed, expected %d misses, got %d", expectedMisses, cache.Misses)
	}
}

func TestProgramCache(t *testing.T) {
	app := &starform.AppObject{
		Name: "test",
	}

	t.Run("total-reuse", func(t *testing.T) {
		cache := &testScriptCache{}
		opts := &starform.ScriptSetOptions{
			App:   app,
			Cache: cache,
		}
		sources := []starform.ScriptSource{&testScriptSource{
			name: "test.star",
			content: `
				def init():
					print('foo')
			`,
		}}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := scripts.LoadSources(context.Background(), sources); err != nil {
			t.Fatal(err)
		}
		if err := scripts.LoadSources(context.Background(), sources); err != nil {
			t.Fatal(err)
		}
		if cache.Misses > 1 {
			t.Error("unexpected cache miss")
		}
	})

	t.Run("partial-reuse", func(t *testing.T) {
		cache := &testScriptCache{}
		opts := &starform.ScriptSetOptions{
			App:   app,
			Cache: cache,
		}
		sets := [][]starform.ScriptSource{{
			&testScriptSource{
				name: "222.star",
				content: `
					bar = 'bar'
					def init():
						print('foo')
				`,
			}, &testScriptSource{
				name: "111.star",
				content: `
					load('222.star', 'bar')
					def init():
						print(bar)
				`,
			},
		}, {
			&testScriptSource{
				name: "222.star",
				content: `
					bar = 'baz'
					def init():
						print('foo')
				`,
			}, &testScriptSource{
				name: "111.star",
				content: `
					load('222.star', 'bar')
					def init():
						print(bar)
				`,
			},
		}}
		for _, set := range sets {
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := scripts.LoadSources(context.Background(), set); err != nil {
				t.Fatal(err)
			}
		}
		// same name but different content leads to a cache miss.
		if cache.Misses != 3 {
			t.Error("unexpected cache misses")
		}
	})
}

func TestObserverTypes(t *testing.T) {
	app := &starform.AppObject{
		Name: "app",
	}
	tests := []struct {
		name        string
		observer    string
		expectedErr string
		expectedLog string
	}{{
		name:        "builtin",
		observer:    "print",
		expectedLog: "<Event foo>\n",
	}, {
		name:        "lambda",
		observer:    "lambda event: print('handled event')",
		expectedLog: "handled event\n",
	}, {
		name:        "non-callable",
		observer:    `"interloper"`,
		expectedErr: "cannot load script: observe: for parameter 2: got string, want callable",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := &testLogger{}
			opts := &starform.ScriptSetOptions{
				App:    app,
				Logger: logger,
			}
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			sources := []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: fmt.Sprintf(
					`
						def init():
							app.observe('foo', %s)
					`,
					test.observer,
				),
			}}
			if err := scripts.LoadSources(context.Background(), sources); err != nil {
				if test.expectedErr == "" {
					t.Errorf("unexpected error: %q", err)
				} else if err.Error() != test.expectedErr {
					t.Errorf("unexpected error: expected %q but got %q", test.expectedErr, err)
				}
			} else if test.expectedErr != "" {
				t.Errorf("expected error")
			}

			event := &starform.EventObject{Name: "foo"}
			if err := scripts.Handle(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if actualLog := logger.String(); actualLog != test.expectedLog {
				t.Errorf("output error: expected %v got %v", test.expectedLog, actualLog)
			}
		})
	}
}

func TestEventHandling(t *testing.T) {
	const expectedLog = "foo\n1\non_foo_3\nbar\nTrue\n"

	logger := &testLogger{}
	opts := &starform.ScriptSetOptions{
		App: &starform.AppObject{
			Name: "app",
		},
		Logger: logger,
	}
	scripts, err := starform.NewScriptSet(opts)
	if err != nil {
		t.Fatal(err)
	}
	sources := []starform.ScriptSource{&testScriptSource{
		name: "111.star",
		content: `
			def init():
				app.observe('foo', on_foo_1)
				app.observe('foo', on_foo_2)
				app.observe('bar', on_bar)

			def on_foo_1(event):
				print(event.name)

			def on_foo_2(event):
				print(event.foo)

			def on_bar(event):
				print(event.name)
				print(event.bar)
		`,
	}, &testScriptSource{
		name: "222.star",
		content: `
			def init():
				app.observe('foo', on_foo_3)

			def on_foo_3(event):
				print("on_foo_3")
		`,
	}}
	if err := scripts.LoadSources(context.Background(), sources); err != nil {
		t.Fatal(err)
	}

	event := &starform.EventObject{
		Name: "foo",
		Attrs: starlark.StringDict{
			"foo": starlark.MakeInt(1),
		},
	}
	if err := scripts.Handle(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	event = &starform.EventObject{
		Name: "bar",
		Attrs: starlark.StringDict{
			"bar": starlark.True,
		},
	}
	if err := scripts.Handle(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if actualLog := logger.String(); actualLog != expectedLog {
		t.Errorf("output error: expected %v got %v", expectedLog, actualLog)
	}
}

func TestEventHandlingFailPropagation(t *testing.T) {
	const expected = "fail: oh no!"

	opts := &starform.ScriptSetOptions{
		App: &starform.AppObject{
			Name: "app",
		},
	}
	scripts, err := starform.NewScriptSet(opts)
	if err != nil {
		t.Fatal(err)
	}
	sources := []starform.ScriptSource{&testScriptSource{
		name: "test.star",
		content: `
			def init():
				app.observe('foo', on_foo)

			def on_foo(event):
				fail('oh no!')
		`,
	}}
	if err := scripts.LoadSources(context.Background(), sources); err != nil {
		t.Fatal(err)
	}

	event := &starform.EventObject{Name: "foo"}
	if err := scripts.Handle(context.Background(), event); err == nil {
		t.Fatal("expected error")
	} else if err.Error() != expected {
		t.Errorf("incorrect error: expected %s but got %v", expected, err)
	}
}

func TestObserveAvailability(t *testing.T) {
	runScript := func(name, code string) error {
		opts := &starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "app",
			},
		}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			return err
		}
		sources := []starform.ScriptSource{&testScriptSource{
			name:    name + ".star",
			content: code,
		}}
		if err := scripts.LoadSources(context.Background(), sources); err != nil {
			return err
		}
		event := &starform.EventObject{Name: "foo"}
		if err := scripts.Handle(context.Background(), event); err != nil {
			return err
		}
		return nil
	}

	tests := []struct {
		name      string
		available bool
		code      string
	}{{
		name:      "toplevel",
		available: false,
		code: `
			def on_foo(event):
				fail('unexpectedly called')

			app.observe('foo', on_foo)
		`,
	}, {
		name:      "init",
		available: true,
		code: `
			def init():
				app.observe('foo', on_foo)

			def on_foo(event):
				pass
		`,
	}, {
		name:      "event",
		available: false,
		code: `
			def init():
				app.observe('foo', on_foo)

			def on_foo(event):
				app.observe('bar', on_bar)
				fail('unexpectedly called')

			def on_bar(event):
				pass
		`,
	}, {
		name:      "capture",
		available: false,
		code: `
			def init():
				observe = app.observe

				def on_foo(event):
					observe('bar', on_bar)
					fail('unexpectedly called')

				observe('foo', on_foo)

			def on_bar(event):
				pass
		`,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runScript(test.name, test.code)
			if test.available && err != nil {
				t.Fatal(err)
			} else if !test.available && err == nil {
				t.Fatalf("expected ErrUnavailable, got success")
			} else if !test.available && !errors.Is(err, starform.ErrUnavailable) {
				t.Fatalf("expected ErrUnavailable, got %v", err)
			}
		})
	}
}

func TestObserverFreezing(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{{
		name: "no-shared-state-by-closure",
		script: `
			def init():
				shared_state = {'a': 1}

				def on_foo(event):
					shared_state['a'] = 100
					fail('state change leaked')

				app.observe('foo', on_foo)
		`,
	}, {
		name: "no-shared-state-by-toplevel",
		script: `
			shared_state = {'a': 1}

			def init():
				app.observe('foo', on_foo)

			def on_foo(event):
				shared_state['a'] = 2
				fail('state change leaked')
		`,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := &starform.ScriptSetOptions{
				App: &starform.AppObject{
					Name: "app",
				},
			}
			scripts, err := starform.NewScriptSet(opts)
			if err != nil {
				t.Fatal(err)
			}
			sources := []starform.ScriptSource{&testScriptSource{
				name:    "test.star",
				content: test.script,
			}}
			if err := scripts.LoadSources(context.Background(), sources); err != nil {
				t.Fatal(err)
			}
			event := &starform.EventObject{Name: "foo"}
			if err := scripts.Handle(context.Background(), event); err == nil {
				t.Error("expected error")
			} else if err.Error() != "cannot insert into frozen hash table" {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLog(t *testing.T) {
	cache := &testScriptCache{}

	const testProgram = `
		def init():
			app.observe("event", on_event)
			print("print in init")
			debug("debug in init")

		def on_event(event):
			print("print handling event")
			debug("debug handling event")

		print("print at toplevel")
		debug("debug at toplevel")
	`
	expectedEntries := []starform.LogEntry{{
		Level:     starform.PrintLevel,
		EventName: starform.LoadEventName,
		Message:   "print at toplevel",
		Line:      10,
	}, {
		Level:     starform.DebugLevel,
		EventName: starform.LoadEventName,
		Message:   "debug at toplevel",
		Line:      11,
	}, {
		Level:     starform.PrintLevel,
		EventName: starform.LoadEventName,
		Message:   "print in init",
		Line:      3,
	}, {
		Level:     starform.DebugLevel,
		EventName: starform.LoadEventName,
		Message:   "debug in init",
		Line:      4,
	}, {
		Level:     starform.PrintLevel,
		EventName: "event",
		Message:   "print handling event",
		Line:      7,
	}, {
		Level:     starform.DebugLevel,
		EventName: "event",
		Message:   "debug handling event",
		Line:      8,
	}}
	sources := []testScriptSource{{
		name:    "foo.star",
		content: testProgram,
	}, {
		name:    "bar.star",
		content: testProgram,
	}}
	for _, source := range sources {
		expectedEntries := expectedEntries // Shadow to keep the original entries available for next round.
		logger := &testLogger{
			format: func(entry starform.LogEntry) string {
				if len(expectedEntries) == 0 {
					t.Errorf("unexpected log entry: %v", entry)
				}
				expectedEntry := expectedEntries[0]
				expectedEntries = expectedEntries[1:]
				if expectedEntry.Level != entry.Level {
					t.Errorf("unexpected log level: want %v got %v", expectedEntry.Level, entry.Level)
				}
				if expectedEntry.EventName != entry.EventName {
					t.Errorf("unexpected event name: want %s got %s", expectedEntry.EventName, entry.EventName)
				}
				if expectedEntry.Line != entry.Line {
					t.Errorf("unexpected line reference: want %d got %d", expectedEntry.Line, entry.Line)
				}
				if source.name != entry.Path {
					t.Errorf("unexpected path reference: want %s got %s", source.name, entry.Path)
				}
				if expectedEntry.Message != entry.Message {
					t.Errorf("unexpected log level: want %s got %s", expectedEntry.Message, entry.Message)
				}
				return ""
			},
		}
		opts := &starform.ScriptSetOptions{
			App: &starform.AppObject{
				Name: "app",
			},
			Cache:  cache,
			Logger: logger,
		}
		scripts, err := starform.NewScriptSet(opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := scripts.LoadSources(context.Background(), []starform.ScriptSource{&source}); err != nil {
			t.Fatal(err)
		}
		if err := scripts.Handle(context.Background(), &starform.EventObject{
			Name: "event",
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppAttrs(t *testing.T) {
	const methodSafety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	get_foo := starlark.NewBuiltinWithSafety("get_foo", methodSafety, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
			return nil, err
		}
		return starlark.Value(starlark.String("foo")), nil
	})
	get_bar := starlark.NewBuiltinWithSafety("get_bar", methodSafety, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
			return nil, err
		}
		return starlark.Value(starlark.String("bar")), nil
	})
	customObserve := starlark.NewBuiltinWithSafety("observe", methodSafety, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Do nothing.
		return starlark.None, nil
	})

	t.Run("non-overloaded", func(t *testing.T) {
		app := &starform.AppObject{
			Name: "test",
			Methods: []*starlark.Builtin{
				get_foo,
				get_bar,
			},
		}

		t.Run("load", func(t *testing.T) {
			set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
				App: app,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: `
					def on_foo(event):
						fail("unreachable")

					# observe is not available during module loading.
					test.observe("foo", on_foo)
					fail("unreachable")
				`,
			}})
			if err == nil {
				t.Error("expected error, got success")
			} else if !errors.Is(err, starform.ErrUnavailable) {
				t.Errorf("expected %v, got %v", starform.ErrUnavailable, err)
			}
		})

		t.Run("init", func(t *testing.T) {
			const expectedLog = "foo handled\n"

			logger := &testLogger{}
			set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
				App:    app,
				Logger: logger,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: `
					def on_foo(event):
						print(test.get_foo(), "handled")

					def init():
						test.observe("foo", on_foo)
				`,
			}})
			if err != nil {
				t.Error(err)
			}

			event := &starform.EventObject{Name: "foo"}
			err = set.Handle(context.Background(), event)
			if err != nil {
				t.Error(err)
			}
			if log := logger.String(); log != expectedLog {
				t.Errorf("expected %v, got %v", expectedLog, log)
			}
		})

		t.Run("event-handling", func(t *testing.T) {
			set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
				App: app,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: `
					def init():
						test.observe("foo", on_foo)

					def on_foo(event):
						# observe is not available during event handling.
						test.observe("bar", on_bar)
						fail("unreachable")

					def on_bar(event):
						fail("unreachable")
				`,
			}})
			if err != nil {
				t.Error(err)
			}

			event := &starform.EventObject{Name: "foo"}
			err = set.Handle(context.Background(), event)
			if err == nil {
				t.Error("expected error, got success")
			} else if !errors.Is(err, starform.ErrUnavailable) {
				t.Errorf("expected %v, got %v", starform.ErrUnavailable, err)
			}
		})
	})

	t.Run("overloaded", func(t *testing.T) {
		testApp := &starform.AppObject{
			Name: "test",
			Methods: []*starlark.Builtin{
				get_bar,
				get_foo,
				customObserve, // Overload observe.
			},
		}

		t.Run("load", func(t *testing.T) {
			set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
				App: testApp,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: `
					def on_foo(event):
						fail("unreachable")

					# Custom fields are not available outside of event handling.
					test.observe("foo", on_foo)
					fail("unreachable")
				`,
			}})
			if err == nil {
				t.Error("expected error, got success")
			} else if !errors.Is(err, starform.ErrUnavailable) {
				t.Errorf("expected %v, got %v", starform.ErrUnavailable, err)
			}
		})

		t.Run("init", func(t *testing.T) {
			set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
				App: testApp,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
				name: "test.star",
				content: `
					def on_foo(event):
						fail("unreachable")

					def init():
						# Custom fields are not available outside of event handling.
						test.observe("foo", on_foo)
						fail("unreachable")
				`,
			}})
			if err == nil {
				t.Error("expected error, got success")
			} else if !errors.Is(err, starform.ErrUnavailable) {
				t.Errorf("expected %v, got %v", starform.ErrUnavailable, err)
			}
		})
	})
}

func TestEventState(t *testing.T) {
	// Example app state
	type startupEventState struct {
		currentOs, targetOs string
	}

	const methodSafety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	getCurrentOS := starlark.NewBuiltinWithSafety("current_os", methodSafety, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		event := starform.Event(thread)
		if event.Name == "startup" {
			state := event.State.(*startupEventState)
			if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
				return nil, err
			}
			return starlark.String(state.currentOs), nil
		}
		return nil, starform.ErrUnavailable
	})
	getTargetOS := starlark.NewBuiltinWithSafety("target_os", methodSafety, func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		event := starform.Event(thread)
		if event.Name == "startup" {
			state := event.State.(*startupEventState)
			if err := thread.AddAllocs(starlark.StringTypeOverhead); err != nil {
				return nil, err
			}
			return starlark.String(state.targetOs), nil
		}
		return nil, starform.ErrUnavailable
	})
	app := &starform.AppObject{
		Name: "exec",
		Methods: []*starlark.Builtin{
			getCurrentOS,
			getTargetOS,
		},
	}

	tests := []struct {
		targetOs, currentOs string
		expectError         bool
	}{{
		targetOs:    "linux",
		currentOs:   "linux",
		expectError: false,
	}, {
		targetOs:    "android",
		currentOs:   "linux",
		expectError: true,
	}, {
		targetOs:    "windows",
		currentOs:   "linux",
		expectError: false,
	}}
	for _, test := range tests {
		set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
			App: app,
		})
		if err != nil {
			t.Fatal(err)
		}
		err = set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
			name: "test.star",
			content: `
				def on_startup(event):
					current_os = exec.current_os()
					target_os = exec.target_os()
					if current_os != target_os:
						if target_os == "windows" and current_os == "linux":
							print("use wine for this!")
						else:
							fail("cannot run app for %s on %s" % (target_os, current_os))

				def init():
					exec.observe("startup", on_startup)
			`,
		}})
		if err != nil {
			t.Error(err)
		}
		event := &starform.EventObject{
			Name: "startup",
			State: &startupEventState{
				currentOs: test.currentOs,
				targetOs:  test.targetOs,
			},
		}
		err = set.Handle(context.Background(), event)
		if test.expectError && err == nil {
			t.Error("expected error")
		} else if !test.expectError && err != nil {
			t.Error(err)
		}
	}
}

func TestRecursiveEvent(t *testing.T) {
	handle := starlark.NewBuiltin("handle", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eventName string
		var n starlark.Int
		if err := starlark.UnpackPositionalArgs("handle", args, kwargs, 2, &eventName, &n); err != nil {
			return nil, err
		}

		event := starform.Event(thread)
		set := event.State.(*starform.ScriptSet)
		err := set.Handle(thread.Context(), &starform.EventObject{
			Name:  eventName,
			State: event.State,
			Attrs: starlark.StringDict{
				"n": n,
			},
		})
		if err != nil {
			return nil, err
		}

		if event != starform.Event(thread) {
			return nil, fmt.Errorf("parent event not restored after recursion")
		}

		return starlark.None, nil
	})

	set, err := starform.NewScriptSet(&starform.ScriptSetOptions{
		App: &starform.AppObject{
			Name:    "test",
			Methods: []*starlark.Builtin{handle},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	set.LoadSources(context.Background(), []starform.ScriptSource{&testScriptSource{
		name: "test.star",
		content: `
			def init():
				test.observe('even', on_even)
				test.observe('odd', on_odd)

			def on_even(event):
				if (event.n % 2) != 0:
					fail("expected even number, got", event.n)
				if event.n > 0:
					test.handle('odd', event.n-1)

			def on_odd(event):
				if (event.n % 2) == 0:
					fail("expected odd number, got", event.n)
				if event.n > 0:
					test.handle('even', event.n-1)
		`,
	}})

	err = set.Handle(context.Background(), &starform.EventObject{
		Name:  "even",
		State: set,
		Attrs: starlark.StringDict{
			"n": starlark.MakeInt(100),
		},
	})
	if err != nil {
		t.Error(err)
	}
}
