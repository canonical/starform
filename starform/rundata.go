package starform

import (
	"errors"
	"fmt"

	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"

type runData struct {
	eventName string // TODO(kcza): Generalise this to include some ID for more efficient comparison.
}

const runDataLocalKey = "starform.runData"

var errMissingRunData = errors.New("runData missing")

func getRunData(thread *starlark.Thread) (*runData, error) {
	ret, ok := thread.Local(runDataLocalKey).(*runData)
	if !ok {
		return nil, errMissingRunData
	}
	return ret, nil
}
