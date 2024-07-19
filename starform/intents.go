package starform

import (
	"fmt"
	"sync"

	"github.com/canonical/starlark/starlark"
)

type intentStore struct {
	mu      sync.Mutex
	entries []interface{}
}

const intentStoreLocalKey = "starform-intents"

var errIntentsLocalMissing = fmt.Errorf("local %q missing", intentStoreLocalKey)

func DeclareIntent(thread *starlark.Thread, intent interface{}) error {
	const safety = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
	if err := starlark.CheckSafety(thread, safety); err != nil {
		return err
	}

	event := Event(thread)
	if event.Name == loadEventName {
		return ErrUnavailable
	}

	return declareIntent(thread, intent)
}

func declareIntent(thread *starlark.Thread, intent interface{}) error {
	intents, ok := thread.Local(intentStoreLocalKey).(*intentStore)
	if !ok {
		return errIntentsLocalMissing
	}

	intents.mu.Lock()
	defer intents.mu.Unlock()

	entriesAppender := starlark.NewSafeAppender(thread, &intents.entries)
	if err := entriesAppender.Append(intent); err != nil {
		return err
	}
	return nil
}
