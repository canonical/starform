package starform

type ExtensionCache interface{}

type noopExtensionCache struct{}

var DefaultScriptletCache ExtensionCache = &noopExtensionCache{}
