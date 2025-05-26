package alias

type AliasStore interface {
	QueryAliases(input string) ([]AliasEntry, error)
}

type AliasEntry struct {
	Name string
	Cmd  string
}
