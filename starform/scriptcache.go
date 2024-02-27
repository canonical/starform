package starform

type ScriptCache interface {
	sealed() // This will be removed once this interface is stable.
}

type noopScriptCache struct{}

func (*noopScriptCache) sealed() {}

var NoopScriptCache ScriptCache = &noopScriptCache{}
