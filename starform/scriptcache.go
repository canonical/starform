package starform

type ScriptCache interface{}

type noopScriptCache struct{}

var DefaultScriptCache ScriptCache = &noopScriptCache{}
