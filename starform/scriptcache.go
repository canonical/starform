package starform

type ScriptCache interface{}

type noopScriptCache struct{}

var DefaultScriptCache ScriptCache = &noopScriptCache{}

type globalScriptCache struct{}

var GlobalScriptCache ScriptCache = &globalScriptCache{}
