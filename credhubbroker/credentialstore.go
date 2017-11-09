package credhubbroker

import "github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"

//go:generate counterfeiter -o fakes/credentialstore.go . CredentialStore
type CredentialStore interface {
	Set(key string, value interface{}) error
	Delete(key string) error
	Authenticate() error
	AddPermissions(credentialName string, perms []permissions.Permission) ([]permissions.Permission, error)
}
