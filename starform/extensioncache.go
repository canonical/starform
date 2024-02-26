package starform

type ExtensionCache interface {
	private()
}

type noopExtensionCache struct{}

func (*noopExtensionCache) private() {}

var NoopExtensionCache ExtensionCache = &noopExtensionCache{}
