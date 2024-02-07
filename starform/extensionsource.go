package starform

type ExtensionLoader interface {
	Load(name string) ([]ExtensionSource, error)
}

type ExtensionSource interface {
	Path() string
	Content() ([]byte, error)
}
