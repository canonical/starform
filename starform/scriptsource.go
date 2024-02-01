package starform

type ScriptLoader interface {
	Load(name string) ([]ScriptSource, error)
}

type ScriptSource interface {
	Name() string
	Content() (interface{}, error)
	Hash() string
}
