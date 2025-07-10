# Starform

[![CI tests](https://github.com/canonical/starform/actions/workflows/tests.yml/badge.svg)](https://github.com/canonical/starform/actions/workflows/tests.yml)
[![GoDoc](https://godoc.org/github.com/canonical/starform?status.svg)](https://godoc.org/github.com/canonical/starform)
<!-- TODO: [![Coverage Status](https://coveralls.io/repos/github/canonical/starform/badge.svg?branch=master)](https://coveralls.io/github/canonical/starform?branch=master) -->
<!-- TODO: [![Go Report Card](https://goreportcard.com/badge/github.com/canonical/starform)](https://goreportcard.com/report/github.com/canonical/starform) -->

This repository provides the `starform` Go package, a library to help easily and uniformly integrate event-based Starlark scriptlet APIs into applications.

## What is Starform?

Starform is a library which allows users to customize and extend applications with procedural configuration.
Its design allows each application to define an API that can be as domain-specific as needed whilst maintaining a consistent overall syntax, structure and set of idioms.
Several aspects of how to approach this problem correctly and conveniently are taken into account in the implementation and in the APIs that are made available.

Starform provides a scripting interface which exposes events.
Users then write [safe Starlark](https://github.com/canonical/starlark) scriptlets reacting to these events, expressing what they want the application to do in each scenario.

The output of a scriptlet is a target for how the user wishes the system to be or to act – no changes are applied to the host system until after scriptlet execution is complete.
This contrasts with classical scripting where changes are made during script execution.
Scriptlets follow the same principle of regular declarative configuration where after reading a config file, contents are validated, amended and only then, if the system deems them reasonable are their changes applied.
Whereas declarative configuration allows a single intent to be specified at startup, starform allows many intents to be specified in reaction to the system's current state, through the use of custom event-handlers.

Starform helps enforce a uniform scriptlet API across different systems.
The user can hence concentrate on building domain-specific knowledge rather than having to learn a new configuration dialect every time.

## Usage skeleton

Here's a simple skeleton you can use to get Starform working.
There are two main parts.

Firstly, you initialize the scriptlet environment (this only needs to happen once)–

```go
app := &starform.AppObject{
    Name:    "my_app",
    Methods: []*starlark.Builtin{
        // Your domain-specific methods go here.
    }
}
scriptSet := starform.NewScriptSet(&starform.ScriptSetOptions{
    App:            app,
    RequiredSafety: starlark.MemSafe,
    MaxAllocs:      10 * 1024 * 1024,
})

err := scriptSet.LoadSources(context.TODO(), []starform.ScriptSource{
    // Your sources go here.
})
if err != nil {
    log.Fatalf("failed to load sources: %v", err)
}
```

Secondly, you handle events (this can be repeated many times)–

```go
err = scriptSet.Handle(context.TODO(), &starform.EventObject{
    Name:  "my_event",
    Attrs: starlark.StringDict {
        // Your event information here.
    },
})
if err != nil {
    log.Fatalf("failed to handle event: %v", err)
}
```

## Documentation

The documentation for this package can be found on [pkg.go.dev](https://pkg.go.dev/github.com/canonical/starform).

## License

This project is licensed under the LGPLv3 License – see the [LICENSE](LICENSE) file for details.
