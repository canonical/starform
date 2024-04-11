package starform

import (
	"fmt"

	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"
var InitEventName = "<init>"

type runData struct {
	eventName string
}

const runDataLocalKey = "starform.runData"

func getRunData(thread *starlark.Thread) (*runData, error) {
	ret, ok := thread.Local(runDataLocalKey).(*runData)
	if !ok {
		return nil, fmt.Errorf("local key %q has been overwritten", runDataLocalKey)
	}
	return ret, nil
}

func putRunData(thread *starlark.Thread, data *runData) {
	thread.SetLocal(runDataLocalKey, data)
}
