package check

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFormattingIsReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	original := []byte("package sample\nfunc Value()int{return 1}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Formatting(root); err == nil {
		t.Fatal("accepted unformatted source")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("formatting check changed the source")
	}
	if err := os.WriteFile(path, []byte("package sample\n\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent", "ignored.go"), []byte("not Go source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Formatting(root); err != nil {
		t.Fatal(err)
	}
}

func TestRunStopsBeforeCommandsOnInvalidContracts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "specs"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Run(context.Background(), root, &output, false); err == nil {
		t.Fatal("accepted an empty spec collection")
	}
	if bytes.Contains(output.Bytes(), []byte("go vet")) {
		t.Fatal("ran later checks after contract failure")
	}
}

func TestCanceledCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := Run(ctx, t.TempDir(), &output, false); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if output.Len() != 0 {
		t.Fatal("started a step after cancellation")
	}
}
