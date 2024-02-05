package starform

type ScriptLoader interface {
	Load(name string) ([]ScriptSource, error)
}

type ScriptSource interface {
	Path() string
	Content() (interface{}, error)
	Hash() interface{}
}
