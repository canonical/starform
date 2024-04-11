package starform_test

import (
	"testing"

	"github.com/canonical/starform/starform"
)

func TestAppObjectAsStarlarkValue(t *testing.T) {
	const appObjectName = "testObject"

	appObject := starform.NewAppObject(appObjectName)
	appObject.Freeze()

	if !bool(appObject.Truth()) {
		t.Errorf("app object should be truthy")
	}
	if appObjectString := appObject.String(); appObjectString != appObjectName {
		t.Errorf("incorrect string representation: expected %q but got %q", appObjectName, appObjectString)
	}
	if appObjectType := appObject.Type(); appObjectType != appObjectName {
		t.Errorf("incorrect type representation: expected %q but got %q", appObjectName, appObjectType)
	}
	if _, err := appObject.Hash(); err == nil {
		t.Errorf("app object should not be hashable")
	}
}
