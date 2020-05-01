package language

type Deployer interface {
	Deploy(source, destination string) error
}

type Inspector interface {
	GetID(source string) (string, error)
}

type Bindings interface {
	Deployer
	Inspector
}
