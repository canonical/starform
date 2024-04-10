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

func CheckAvailable(thread *starlark.Thread, availableDuring []string) error {
	rd, err := getRunData(thread)
	if err != nil {
		return err
	}
	for _, event := range availableDuring {
		if event == rd.eventName {
			return nil
		}
	}
	return fmt.Errorf("feature unavailable during %s", rd.eventName)
}
