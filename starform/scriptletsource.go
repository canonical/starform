package starform

type ScriptletLoader interface {
	Load(name string) ([]ScriptletSource, error)
}

type ScriptletSource interface {
	Path() string
	Content() (interface{}, error)
	Hash() (interface{}, error)
}
