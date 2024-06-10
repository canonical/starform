package starform

import (
	"errors"

	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"

type runData struct {
	eventName string // TODO(kcza): Generalise this to include some ID for more efficient comparison.
}

const runDataLocalKey = "starform-run-data"

var errRunDataMissing = errors.New("starform internal data missing")

func getRunData(thread *starlark.Thread) (*runData, error) {
	ret, ok := thread.Local(runDataLocalKey).(*runData)
	if !ok {
		return nil, errRunDataMissing
	}
	return ret, nil
}
