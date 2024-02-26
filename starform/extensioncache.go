package starform

type ExtensionCache interface {
	private() // This will be removed once this interface is stabilised.
}

type noopExtensionCache struct{}

func (*noopExtensionCache) private() {}

var NoopExtensionCache ExtensionCache = &noopExtensionCache{}
