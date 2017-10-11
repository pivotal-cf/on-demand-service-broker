package credhubbroker

//go:generate counterfeiter -o fakes/credentialstore.go . CredentialStore
type CredentialStore interface {
	Set(key string, value interface{}) error
}
