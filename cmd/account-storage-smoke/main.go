// account-storage-smoke exercises only an explicit synthetic Keychain namespace.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/credentials"
)

const first = "my-bilibili synthetic credential one"
const second = "my-bilibili synthetic credential two"

type receipt struct {
	Phase          string   `json:"phase"`
	Passed         bool     `json:"passed"`
	Checks         []string `json:"checks"`
	FailedCheck    string   `json:"failed_check,omitempty"`
	CleanupPending bool     `json:"cleanup_pending"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func(namespace string) (account.Store, error) { return credentials.NewSynthetic(namespace) }))
}
func run(args []string, out, diagnostics io.Writer, newStore func(string) (account.Store, error)) int {
	fs := flag.NewFlagSet("account-storage-smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	namespace := fs.String("namespace", "", "synthetic namespace")
	phase := fs.String("phase", "", "write or restore-delete")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *namespace == "" || (*phase != "write" && *phase != "restore-delete") {
		fmt.Fprintln(diagnostics, "usage: account-storage-smoke --namespace <32 lowercase hex> --phase write|restore-delete")
		return 2
	}
	store, err := newStore(*namespace)
	if err != nil {
		fmt.Fprintln(diagnostics, "invalid synthetic storage namespace")
		return 2
	}
	r := exercise(store, *phase)
	if json.NewEncoder(out).Encode(r) != nil {
		fmt.Fprintln(diagnostics, "could not write storage check receipt")
		return 1
	}
	if !r.Passed {
		return 1
	}
	return 0
}
func exercise(store account.Store, phase string) receipt {
	r := receipt{Phase: phase, Checks: []string{}, CleanupPending: true}
	check := func(name string, ok bool) bool {
		if !ok {
			r.FailedCheck = name
			return false
		}
		r.Checks = append(r.Checks, name)
		return true
	}
	missing := func() bool { _, err := store.Load(); return err == account.Missing }
	matches := func(value string) bool {
		m, err := store.Load()
		return err == nil && bytes.Equal(m.Bytes(), []byte(value))
	}
	if phase == "write" {
		if !check("initially-missing", missing()) {
			return r
		}
		if !check("save-one", store.Save(account.NewMaterial([]byte(first))) == nil) {
			return r
		}
		if !check("read-one", matches(first)) {
			return r
		}
		if !check("replace-two", store.Save(account.NewMaterial([]byte(second))) == nil) {
			return r
		}
		if !check("read-two", matches(second)) {
			return r
		}
	} else {
		if !check("restore-two", matches(second)) {
			return r
		}
		if !check("delete", store.Delete() == nil) {
			return r
		}
		if !check("missing-after-delete", missing()) {
			return r
		}
		if !check("delete-again", store.Delete() == nil) {
			return r
		}
		r.CleanupPending = false
	}
	r.Passed = true
	return r
}
