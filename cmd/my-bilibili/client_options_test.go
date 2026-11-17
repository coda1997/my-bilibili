package main

import (
	"path/filepath"
	"testing"

	"my-bilibili/internal/catalog"
)

func TestClientModeAndVerificationOptionsAreExplicit(t *testing.T) {
	plan := filepath.Join(t.TempDir(), "plan.json")
	for _, test := range []struct {
		args           []string
		e2e            bool
		scene          string
		fixture, valid bool
	}{
		{nil, false, "", false, true},
		{[]string{"--fixture"}, false, "", true, true},
		{nil, true, "", true, true},
		{nil, true, "search", false, true},
		{[]string{"--verify-readonly", plan}, false, "", false, true},
		{[]string{"--verify-readonly", plan}, true, "", false, false},
		{[]string{"--fixture", "--verify-readonly", plan}, false, "", false, false},
		{[]string{"--fixture"}, true, "search", false, false},
		{nil, false, "search", false, false},
		{[]string{"--verify-readonly", "relative.json"}, false, "", false, false},
		{[]string{"--endpoint", "https://example.test/"}, false, "", false, false},
		{[]string{"unexpected"}, false, "", false, false},
	} {
		got, err := parseClientOptions(test.args, test.e2e, test.scene)
		if test.valid {
			if err != nil || got.fixture != test.fixture || got.scenario != test.scene {
				t.Fatal("valid explicit mode rejected")
			}
		} else if err != catalog.Invalid {
			t.Fatal("invalid mode accepted")
		}
	}
}
