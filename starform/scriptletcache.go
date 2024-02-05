package starform

type ScriptletCache interface{}

type noopScriptletCache struct{}

var DefaultScriptletCache ScriptletCache = &noopScriptletCache{}
