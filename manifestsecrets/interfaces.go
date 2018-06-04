package manifestsecrets

//go:generate counterfeiter -o fakes/fake_bulk_getter.go . BulkGetter

type BulkGetter interface {
	BulkGet([][]byte) (map[string]string, error)
}

//go:generate counterfeiter -o fakes/fake_matcher.go . Matcher

type Matcher interface {
	Match(manifest []byte) ([][]byte, error)
}
