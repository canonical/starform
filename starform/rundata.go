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

const runDataKey = "starform.RunData"

func getRunData(thread *starlark.Thread) (*runData, error) {
	ret, ok := thread.Local(runDataKey).(*runData)
	if !ok {
		return nil, fmt.Errorf("local key %q has been overwritten", runDataKey)
	}
	return ret, nil
}

func putRunData(thread *starlark.Thread, data *runData) {
	thread.SetLocal(runDataKey, data)
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
