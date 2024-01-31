package starform

import "github.com/canonical/starlark/starlark"

type Extension struct {
	Name    string
	modules []starlark.StringDict
}
