package starform

import (
	"crypto/sha512"
	"fmt"

	"github.com/canonical/starlark/starlark"
)

var LoadEventName = "<load>"

type runData struct {
	eventName        string // TODO(kcza): Generalise this to include some ID for more efficient comparison.
	pathByProgramKey map[[sha512.Size384]byte]string
}

const runDataLocalKey = "starform.runData"

var errRunDataMissing = fmt.Errorf("%s missing", runDataLocalKey)

func getRunData(thread *starlark.Thread) (*runData, error) {
	ret, ok := thread.Local(runDataLocalKey).(*runData)
	if !ok {
		return nil, errRunDataMissing
	}
	return ret, nil
}
