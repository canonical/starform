package starform

type Loader interface {
	Load(nameOrPath string) ([]ScriptSource, error)
}

type ScriptSource interface {
	Name() string
	Content() (interface{}, error)
	Hash() string
}
