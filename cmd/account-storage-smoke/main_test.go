package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/credentials"
)

const namespace = "0123456789abcdef0123456789abcdef"

type fakeStore struct {
	value   account.Material
	present bool
	calls   []string
	fail    string
}

func (s *fakeStore) Load() (account.Material, error) {
	s.calls = append(s.calls, "load")
	if s.fail == "load" {
		return account.Material{}, errors.New("PRIVATE_NATIVE_ERROR")
	}
	if !s.present {
		return account.Material{}, account.Missing
	}
	return s.value, nil
}
func (s *fakeStore) Save(m account.Material) error {
	s.calls = append(s.calls, "save")
	if s.fail == "save" {
		return errors.New("PRIVATE_NATIVE_ERROR")
	}
	s.value = m
	s.present = true
	return nil
}
func (s *fakeStore) Delete() error {
	s.calls = append(s.calls, "delete")
	if s.fail == "delete" {
		return errors.New("PRIVATE_NATIVE_ERROR")
	}
	s.value = account.Material{}
	s.present = false
	return nil
}
func factory(s *fakeStore) func(string) (account.Store, error) {
	return func(n string) (account.Store, error) {
		if _, err := credentials.NewSynthetic(n); err != nil {
			return nil, err
		}
		return s, nil
	}
}
func TestArgumentsCannotChooseRealStoreOrSecrets(t *testing.T) {
	for _, args := range [][]string{nil, {"--phase", "write"}, {"--namespace", namespace}, {"--namespace", namespace, "--phase", "bad"}, {"--namespace", namespace, "--phase", "write", "extra"}, {"--namespace", namespace, "--phase", "write", "--secret", "PRIVATE_ARG"}, {"--namespace", "com.coda.mybilibili.account", "--phase", "write"}} {
		s := &fakeStore{}
		var out, diag bytes.Buffer
		if run(args, &out, &diag, factory(s)) != 2 || len(s.calls) != 0 || out.Len() != 0 {
			t.Fatal("invalid args reached storage")
		}
		if strings.Contains(diag.String(), "PRIVATE_ARG") || strings.Contains(diag.String(), "com.coda") {
			t.Fatal("argument value leaked")
		}
	}
}
func TestTwoPhasesRestoreThenDeleteSyntheticItem(t *testing.T) {
	s := &fakeStore{}
	var out, diag bytes.Buffer
	if run([]string{"--namespace", namespace, "--phase", "write"}, &out, &diag, factory(s)) != 0 {
		t.Fatal("synthetic write failed")
	}
	var firstReceipt receipt
	if json.Unmarshal(out.Bytes(), &firstReceipt) != nil || !firstReceipt.Passed || !firstReceipt.CleanupPending || len(firstReceipt.Checks) != 5 || !s.present || !bytes.Equal(s.value.Bytes(), []byte(second)) {
		t.Fatal("write receipt or state incorrect")
	}
	for _, raw := range []string{first, second, namespace} {
		if strings.Contains(out.String(), raw) {
			t.Fatal("synthetic value or namespace leaked")
		}
	}
	out.Reset()
	if run([]string{"--namespace", namespace, "--phase", "restore-delete"}, &out, &diag, factory(s)) != 0 {
		t.Fatal("synthetic restore/delete failed")
	}
	var last receipt
	if json.Unmarshal(out.Bytes(), &last) != nil || !last.Passed || last.CleanupPending || len(last.Checks) != 4 || s.present {
		t.Fatal("delete receipt or state incorrect")
	}
	if diag.Len() != 0 {
		t.Fatal("successful smoke wrote diagnostics")
	}
}
func TestSmokeFailuresRetainCleanupStateWithoutRawErrors(t *testing.T) {
	for _, fail := range []string{"load", "save", "delete"} {
		s := &fakeStore{fail: fail}
		phase := "write"
		if fail == "delete" {
			s.present = true
			s.value = account.NewMaterial([]byte(second))
			phase = "restore-delete"
		}
		var out, diag bytes.Buffer
		if run([]string{"--namespace", namespace, "--phase", phase}, &out, &diag, factory(s)) != 1 {
			t.Fatal("failed smoke passed")
		}
		var r receipt
		if json.Unmarshal(out.Bytes(), &r) != nil || r.Passed || r.FailedCheck == "" || !r.CleanupPending {
			t.Fatal("failure not recorded")
		}
		if strings.Contains(out.String()+diag.String(), "PRIVATE_NATIVE_ERROR") {
			t.Fatal("native error leaked")
		}
	}
	s := &fakeStore{present: true, value: account.NewMaterial([]byte("unrelated synthetic value"))}
	r := exercise(s, "restore-delete")
	if r.Passed || r.FailedCheck != "restore-two" || !s.present {
		t.Fatal("unexpected item was silently deleted")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("PRIVATE_WRITE_ERROR") }
func TestSmokeOutputFailure(t *testing.T) {
	s := &fakeStore{}
	var diag bytes.Buffer
	if run([]string{"--namespace", namespace, "--phase", "write"}, failingWriter{}, &diag, factory(s)) != 1 || strings.Contains(diag.String(), "PRIVATE_WRITE_ERROR") {
		t.Fatal("output failure not safe")
	}
}
