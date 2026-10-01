package roles

type packagePathState uint8

const (
	packagePathNone packagePathState = iota
	packagePathSource
	packagePathSourceSet
	packageNamespace
)

func (s packagePathState) enter(name string) packagePathState {
	if s == packageNamespace {
		return s
	}
	if name == "src" {
		return packagePathSource
	}
	if s == packagePathSource && (name == "main" || name == "test") {
		return packagePathSourceSet
	}
	if s == packagePathSource || s == packagePathSourceSet {
		switch name {
		case "java", "kotlin", "scala", "groovy":
			return packageNamespace
		}
	}
	return packagePathNone
}
