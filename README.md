# Starform 

[![CI tests](https://github.com/canonical/starform/actions/workflows/tests.yml/badge.svg)](https://github.com/canonical/starform/actions/workflows/tests.yml) [![GoDoc](https://godoc.org/github.com/canonical/starform?status.svg)](https://godoc.org/github.com/canonical/starform)

<!-- TODO: [![Coverage Status](https://coveralls.io/repos/github/canonical/starform/badge.svg?branch=master)](https://coveralls.io/github/canonical/starform?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/canonical/starform)](https://goreportcard.com/report/github.com/canonical/starform)  -->

This repository provides the `starform` Go package, a library to help easily and uniformly integrate event-based Starlark scriptlet APIs into applications.

`starform` aims to improve configuration by making it easy for an application to define and handle user *intentions* (i.e. configuration or behaviors) through a constrained, yet expressive scripting interface.
This way, while each API can be as domain-specific as needed, the structure, the construct and the language are consistent.
This way a user can concentrate on building domain-specific knowledge rather than having to learn a new syntax every time.

## What is Starform?

Starform is essentially a wrapper package for the [safe Starlark language interpreter](https://github.com/canonical/starlark). 
It allows developers to provide a scripting interface which exposes events which users can then use to express how they intend for the application to act.

This paradigm of events and intents shouldn't be mistaken for classic scripting—with starform, the output of a scriptlet is a target for how the user wishes the system to be or to act, no changes are made to the host system until after scriptlet execution is complete.
This follows the same principle of regular declarative configuration—after reading a config file, its contents are validated, possibly amended and only then, if the system deems it reasonable are changes enacted.
Whereas declarative configuration allows a single (possibly complex) intent to be specified at startup, starform allows many (possibly complex) intents to be specified in reaction to the system's current state, through the user of different custom event-handlers.

## Example Usage

Here's a simple example of how you might use Starform:

```go
package main

import (
    "context"
    "github.com/canonical/starform"
)

func main() {
    // Create your application object
    app := &staform.AppObject{
        Name:    "my_app_name",
        Methods: []*starlark.Builtin{
            // Your domain-specific methods here
        }
    },

    // Initialize ScriptSet with app object
    scriptSet := starform.NewScriptSet(starform.ScriptSetOptions{
        App:            app,
		RequiredSafety: starlark.MemSafe,
		MaxAllocs:      10 * 1024 * 1024,
    })

    // Load scripts from a source
    err := scriptSet.LoadSources(context.Background(), []starform.ScriptSource{
        // Your sources here
    })
    if err != nil {
        log.Fatalf("failed to load sources: %v", err)
    }

    // Handle an event
    err = scriptSet.Handle(context.Background(), &starform.EventObject{
        Name:  "my_event",
        Attrs: starlark.StringDict {
            // Event information
        },
    })
    if err != nil {
        log.Fatalf("failed to handle event: %v", err)
    }
}
```

_A more exhaustive example is currently being put together and will be linked here._
<!-- For a more exaustive example, see [`vex-go`](https://github.com/canonical/vex-go), a simple tree-sitter-based configurable linter. -->

## Documentation

The documentation for this package can be found on [pkg.go.dev](https://pkg.go.dev/github.com/canonical/starform).

## License

This project is licensed under the LGPLv3 License—see the [LICENSE](LICENSE) file for details.
