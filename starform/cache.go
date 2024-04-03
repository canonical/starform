package starform

type ScriptCache interface {
	private() // This will be removed once this interface is stable.
}

type noopScriptCache struct{}

func (*noopScriptCache) private() {}
