package starform

import (
	"errors"

	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"

type runData struct {
	eventName        string // TODO(kcza): Generalise this to include some ID for more efficient comparison.
	pathByFilename   map[string]string
	observeAvailable bool
	observers        map[string][]starlark.Callable
}

const runDataLocalKey = "starform-run-data"

var errRunDataMissing = errors.New("starform internal data missing")

func getRunData(thread *starlark.Thread) (*runData, error) {
	storedData := thread.Local(runDataLocalKey)
	if storedData == nil {
		return nil, errRunDataMissing
	}
	ret, ok := storedData.(*runData)
	if !ok {
		return nil, errRunDataMissing
	}
	return ret, nil
}

func (rd *runData) GetPath(programKey string) string {
	if path, ok := rd.pathByFilename[programKey]; ok {
		return path
	}
	return "<unknown>"
}
