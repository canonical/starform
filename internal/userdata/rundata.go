package userdata

type RunData struct {
	// Event holds the *starform.EventObject passed into the current Handle
	// call. An interface is used to avoid a dependency cycle.
	Event interface{}
}

const RunDataLocalKey = "starform-event-object"
