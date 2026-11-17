// Package credentials adapts the application's opaque session to protected storage.
package credentials

import (
	"bytes"
	"errors"

	"github.com/egoist/mygo"
	"my-bilibili/internal/account"
)

const service = "com.coda.mybilibili.account"
const key = "web-session-v1"

type backend interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

type nativeBackend struct{ service string }

func (n nativeBackend) Get(k string) ([]byte, error) { return mygo.Secrets.Service(n.service).Get(k) }
func (n nativeBackend) Set(k string, v []byte) error {
	return mygo.Secrets.Service(n.service).Set(k, v)
}
func (n nativeBackend) Delete(k string) error { return mygo.Secrets.Service(n.service).Delete(k) }

// Store is synchronous. Its owner serializes calls; a context cannot cancel a
// native Keychain operation already waiting for system consent.
type Store struct{ backend backend }

// New selects the stable application service and performs no native operation.
func New() *Store { return &Store{backend: nativeBackend{service: service}} }

// NewSynthetic selects only the isolated smoke namespace. It cannot select the
// real session service and never performs a native operation during construction.
func NewSynthetic(namespace string) (*Store, error) {
	if len(namespace) != 32 {
		return nil, account.Invalid
	}
	for _, c := range namespace {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, account.Invalid
		}
	}
	return &Store{backend: nativeBackend{service: service + ".test." + namespace}}, nil
}

func (s *Store) Load() (account.Material, error) {
	value, err := s.backend.Get(key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return account.Material{}, account.Missing
	}
	if err != nil || len(value) == 0 || len(value) > mygo.MaxSecretSize {
		return account.Material{}, account.Storage
	}
	return account.NewMaterial(value), nil
}

// Save reports success only after readback matches the original material. Errors
// never expose native messages, values or hashes, and no plaintext fallback exists.
func (s *Store) Save(material account.Material) error {
	value := material.Bytes()
	if len(value) == 0 || len(value) > mygo.MaxSecretSize {
		return account.Storage
	}
	if s.backend.Set(key, value) != nil {
		return account.Storage
	}
	saved, err := s.backend.Get(key)
	if err != nil || !bytes.Equal(saved, material.Bytes()) {
		return account.Storage
	}
	return nil
}
func (s *Store) Delete() error {
	err := s.backend.Delete(key)
	if err == nil || errors.Is(err, mygo.ErrSecretNotFound) {
		return nil
	}
	return account.Storage
}
