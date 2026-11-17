package credentials

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"my-bilibili/internal/account"
)

const privateSentinel = "SYNTHETIC_PRIVATE_SESSION"

type fakeBackend struct {
	value                     []byte
	present                   bool
	getErr, setErr, deleteErr error
	mismatch, mutate          bool
	calls                     []string
	keys                      []string
}

func (f *fakeBackend) Get(k string) ([]byte, error) {
	f.calls = append(f.calls, "get")
	f.keys = append(f.keys, k)
	if f.getErr != nil {
		return nil, f.getErr
	}
	if !f.present {
		return nil, mygo.ErrSecretNotFound
	}
	if f.mismatch {
		return []byte("other synthetic value"), nil
	}
	return f.value, nil
}
func (f *fakeBackend) Set(k string, v []byte) error {
	f.calls = append(f.calls, "set")
	f.keys = append(f.keys, k)
	if f.setErr != nil {
		return f.setErr
	}
	if f.mutate && len(v) > 0 {
		v[0] = 'X'
	}
	f.value = bytes.Clone(v)
	f.present = true
	return nil
}
func (f *fakeBackend) Delete(k string) error {
	f.calls = append(f.calls, "delete")
	f.keys = append(f.keys, k)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.present = false
	f.value = nil
	return nil
}
func assertSafe(t *testing.T, err error, want account.Code) {
	t.Helper()
	if err != want {
		t.Fatal("wrong safe error category")
	}
	if strings.Contains(fmt.Sprint(err), privateSentinel) {
		t.Fatal("native detail leaked")
	}
}
func TestFixedAndSyntheticNamespaces(t *testing.T) {
	if got := New().backend.(nativeBackend).service; got != "com.coda.mybilibili.account" {
		t.Fatal("real store identity changed")
	}
	good := "0123456789abcdef0123456789abcdef"
	s, err := NewSynthetic(good)
	if err != nil || s.backend.(nativeBackend).service != "com.coda.mybilibili.account.test."+good {
		t.Fatal("synthetic service not isolated")
	}
	for _, namespace := range []string{"", strings.Repeat("a", 31), strings.Repeat("a", 33), strings.Repeat("A", 32), strings.Repeat("a", 31) + "/", strings.Repeat("a", 31) + "g", strings.Repeat("a", 29) + "密"} {
		s, err := NewSynthetic(namespace)
		if s != nil || err != account.Invalid {
			t.Fatal("unsafe namespace accepted")
		}
	}
}
func TestLoadMissingFailuresAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    fakeBackend
		want account.Code
	}{
		{"missing", fakeBackend{}, account.Missing},
		{"wrapped-missing", fakeBackend{getErr: fmt.Errorf(privateSentinel+": %w", mygo.ErrSecretNotFound)}, account.Missing},
		{"failure", fakeBackend{getErr: errors.New(privateSentinel)}, account.Storage},
		{"empty", fakeBackend{present: true}, account.Storage},
		{"too-large", fakeBackend{present: true, value: bytes.Repeat([]byte("x"), mygo.MaxSecretSize+1)}, account.Storage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Store{backend: &tc.f}
			m, err := s.Load()
			assertSafe(t, err, tc.want)
			if !m.Empty() {
				t.Fatal("failure published material")
			}
		})
	}
	f := &fakeBackend{present: true, value: bytes.Repeat([]byte("s"), mygo.MaxSecretSize)}
	s := &Store{backend: f}
	m, err := s.Load()
	if err != nil || len(m.Bytes()) != mygo.MaxSecretSize {
		t.Fatal("valid maximum rejected")
	}
	f.value[0] = 'x'
	if m.Bytes()[0] != 's' {
		t.Fatal("native buffer aliases material")
	}
	if len(f.keys) != 1 || f.keys[0] != "web-session-v1" {
		t.Fatal("wrong credential item key")
	}
}
func TestSaveReadbackAndInputIsolation(t *testing.T) {
	for _, size := range []int{0, mygo.MaxSecretSize + 1} {
		f := &fakeBackend{}
		s := &Store{backend: f}
		assertSafe(t, s.Save(account.NewMaterial(bytes.Repeat([]byte("x"), size))), account.Storage)
		if len(f.calls) != 0 {
			t.Fatal("invalid material reached native store")
		}
	}
	for _, tc := range []struct {
		name string
		f    fakeBackend
	}{
		{"set-error", fakeBackend{setErr: errors.New(privateSentinel)}},
		{"read-error", fakeBackend{getErr: errors.New(privateSentinel)}},
		{"read-missing", fakeBackend{getErr: mygo.ErrSecretNotFound}},
		{"mismatch", fakeBackend{mismatch: true}},
		{"mutated-input", fakeBackend{mutate: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := account.NewMaterial([]byte(privateSentinel))
			s := &Store{backend: &tc.f}
			assertSafe(t, s.Save(m), account.Storage)
			if string(m.Bytes()) != privateSentinel {
				t.Fatal("save changed caller material")
			}
		})
	}
	f := &fakeBackend{}
	s := &Store{backend: f}
	for _, v := range [][]byte{[]byte(privateSentinel), bytes.Repeat([]byte("x"), mygo.MaxSecretSize)} {
		if s.Save(account.NewMaterial(v)) != nil {
			t.Fatal("valid save failed")
		}
		if !bytes.Equal(f.value, v) {
			t.Fatal("saved material changed")
		}
	}
	if strings.Join(f.calls, ",") != "set,get,set,get" {
		t.Fatal("save did not verify native readback")
	}
	for _, k := range f.keys {
		if k != "web-session-v1" {
			t.Fatal("save selected another key")
		}
	}
}
func TestDeleteIdempotentAndSafeErrors(t *testing.T) {
	f := &fakeBackend{present: true, value: []byte(privateSentinel)}
	s := &Store{backend: f}
	if s.Delete() != nil || f.present || s.Delete() != nil {
		t.Fatal("delete not idempotent")
	}
	f.deleteErr = mygo.ErrSecretNotFound
	if s.Delete() != nil {
		t.Fatal("missing delete failed")
	}
	f.deleteErr = errors.New(privateSentinel)
	assertSafe(t, s.Delete(), account.Storage)
	for _, k := range f.keys {
		if k != "web-session-v1" {
			t.Fatal("delete selected another key")
		}
	}
}
