# Starform [![CI tests](https://github.com/canonical/starform/actions/workflows/tests.yml/badge.svg)](https://github.com/canonical/starform/actions/workflows/tests.yml) [![GoDoc](https://godoc.org/github.com/canonical/starform?status.svg)](https://godoc.org/github.com/canonical/starform)

<!-- TODO: [![Coverage Status](https://coveralls.io/repos/github/canonical/go-dqlite/badge.svg?branch=master)](https://coveralls.io/github/canonical/go-dqlite?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/canonical/go-dqlite)](https://goreportcard.com/report/github.com/canonical/go-dqlite)  -->

This repository provides the `starform` Go package, a helper library to easily integrate the Starlark language into an application via an event-based scriptlet API.

`starform`'s aim is to make configuration better by making it easy for an application to define and handle user *intentions* (i.e. configuration or behaviors) through a consistent scripting interface.  This way, while each API can be as domain-specific as needed, the structure, the construct and the language are consistent. This way a user can concentrate on building domain-specific knowledge rather than having to learn a new syntax every time.

## What is Starform?

Starform is essentially a "glue" package for the [safe Starlark language interpreter](https://github.com/canonical/starlark). It allows you to implement a scriptlet API that can respond to various application events. This means users can define how they want the application to react to certain triggers in a consistent and predictable manner.

While it could be easy to mistake this paradigm with classic scripting, `staform`'s aim is to make configuration better. Just like configuration is read, validated (and sometimes amended) and then (if possible) applied, Starform's scriptlets follow a similar priciple - handlers are the way an application asks the user for *intents*. Intents are opinions or hints of what the application should do or the next state should look like. As such, during the execution of the script nothing happens. Once the complete picture of what the user intents are, the application an revie, validate and amend the execution plan and then (if possible) execute it.

## Installation

To get started with Starform, you can install it using the following Go command:

```sh
go get github.com/unknown/starform
```

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
        // Logger:         logger, // Optionally add a logger.
		RequiredSafety: starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe,
		MaxAllocs:      10 * 1024 * 1024,
		MaxSteps:       10_000,
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
        State: ..., // App-specific state
        Attrs: starlark.StringDict {
            // Event information
        },
    })
    if err != nil {
        log.Fatalf("failed to handle event: %v", err)
    }
}
```

For a more exaustive example, see [`vex-go`](https://github.com/canonical/vex-go), a simple tree-sitter-based configurable linter.

## Documentation

The documentation for this package can be found on [pkg.go.dev](https://pkg.go.dev/github.com/canonical/starform).

## License

This project is licensed under the LGPLv3 License - see the [LICENSE](LICENSE) file for details.
