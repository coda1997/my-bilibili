package main

import (
	"strings"
	"testing"
)

func TestNativeCompletionRequiresRealCounters(t *testing.T) {
	valid := `{"schema_version":1,"sequence":4,"requests":2,"did_enter":1,"did_exit":1,"dropped":0,"events":[]}`
	read := func(value string) (fullscreenCompletion, error) {
		return nativeCompletion([]node{{Key: "e2e-fullscreen", Label: value}})
	}
	c, err := read(valid)
	if err != nil || c != (fullscreenCompletion{2, 1, 1}) {
		t.Fatalf("valid callback counters rejected: %v %v", c, err)
	}
	for _, bad := range []string{
		strings.Replace(valid, `"did_exit":1,`, "", 1),
		strings.Replace(valid, `"did_exit":1`, `"did_exit":null`, 1),
		strings.Replace(valid, `"did_exit":1`, `"did_exit":0,"did_exit":1`, 1),
		strings.Replace(valid, `"sequence":4`, `"sequence":3`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		strings.Replace(valid, `"requests":2`, `"requests":-1`, 1),
		valid + ` {}`, `null`,
	} {
		if _, err := read(bad); err == nil {
			t.Fatalf("invalid callback evidence accepted: %s", bad)
		}
	}
	if _, err := nativeCompletion(nil); err == nil {
		t.Fatal("missing callback evidence accepted")
	}
	if _, err := nativeCompletion([]node{{Key: "e2e-fullscreen", Label: valid}, {Key: "e2e-fullscreen", Label: valid}}); err == nil {
		t.Fatal("ambiguous callback evidence accepted")
	}
}
