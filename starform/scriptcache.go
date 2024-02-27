package starform

type ScriptCache interface {
	private() // This will be removed once this interface is stabilised.
}

type noopScriptCache struct{}

func (*noopScriptCache) private() {}

var NoopScriptCache ScriptCache = &noopScriptCache{}
