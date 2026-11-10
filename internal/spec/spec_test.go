package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const markdown = "# Example\n\n### TEST-AC-01\nOne observable outcome.\n"

func validDefinition() Definition {
	return Definition{SchemaVersion: 1, SpecID: "test", Tasks: []Task{
		{ID: "T1", Title: "First", WritePaths: []string{"internal/one"}, Acceptance: []string{"TEST-AC-01"}, Checks: []string{"go test ./internal/one"}},
		{ID: "T2", Title: "Second", WritePaths: []string{"internal/two"}, Acceptance: []string{"TEST-AC-01"}, Checks: []string{"go test ./internal/two"}},
	}}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Definition)
		want string
	}{
		{"valid", func(*Definition) {}, ""},
		{"schema", func(d *Definition) { d.SchemaVersion = 2 }, "schema_version"},
		{"empty", func(d *Definition) { d.Tasks = nil }, "nonempty tasks"},
		{"duplicate", func(d *Definition) { d.Tasks[1].ID = "T1" }, "duplicate task"},
		{"unknown dependency", func(d *Definition) { d.Tasks[1].DependsOn = []string{"missing"} }, "unknown task"},
		{"repeated dependency", func(d *Definition) { d.Tasks[1].DependsOn = []string{"T1", "T1"} }, "duplicate dependency"},
		{"self cycle", func(d *Definition) { d.Tasks[0].DependsOn = []string{"T1"} }, "cycle"},
		{"cycle", func(d *Definition) { d.Tasks[0].DependsOn = []string{"T2"}; d.Tasks[1].DependsOn = []string{"T1"} }, "cycle"},
		{"unknown acceptance", func(d *Definition) { d.Tasks[0].Acceptance = []string{"TEST-AC-99"} }, "unknown acceptance"},
		{"empty scope", func(d *Definition) { d.Tasks[0].WritePaths = nil }, "write_paths"},
		{"empty check", func(d *Definition) { d.Tasks[0].Checks = []string{" "} }, "empty check"},
		{"overlap", func(d *Definition) { d.Tasks[1].WritePaths = []string{"internal/one/file.go"} }, "overlap"},
		{"portable case overlap", func(d *Definition) { d.Tasks[1].WritePaths = []string{"Internal/One"} }, "overlap"},
		{"ordered overlap", func(d *Definition) {
			d.Tasks[1].WritePaths = []string{"internal/one"}
			d.Tasks[1].DependsOn = []string{"T1"}
		}, ""},
		{"prefix is not subtree", func(d *Definition) { d.Tasks[1].WritePaths = []string{"internal/one-more"} }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := validDefinition()
			tt.edit(&def)
			err := Validate(def, []byte(markdown))
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Validate = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestAcceptanceCoverage(t *testing.T) {
	for _, text := range []string{markdown + "\n### TEST-AC-02\nMissing coverage.\n", markdown + markdown, "no criteria"} {
		if err := Validate(validDefinition(), []byte(text)); err == nil {
			t.Fatal("accepted missing or duplicate acceptance criteria")
		}
	}
}

func TestTransitiveOrdering(t *testing.T) {
	def := validDefinition()
	def.Tasks[1].DependsOn = []string{"T1"}
	third := def.Tasks[0]
	third.ID, third.DependsOn = "T3", []string{"T2"}
	def.Tasks = append(def.Tasks, third)
	if err := Validate(def, []byte(markdown)); err != nil {
		t.Fatal(err)
	}
}

func TestUnsafePaths(t *testing.T) {
	for _, path := range []string{"", ".", "../secret", "/tmp/a", "a/../b", "a//b", "a/", `C:\secret`, `a\b`, "a/*.go", "a/[x]", "a\nnext", "a/file.", "a/file ", " a/file", "a/NUL", "a/con.txt", "COM1/log"} {
		t.Run(path, func(t *testing.T) {
			if ValidPath(path) {
				t.Fatalf("accepted unsafe path %q", path)
			}
			def := validDefinition()
			def.Tasks[0].WritePaths = []string{path}
			if err := Validate(def, []byte(markdown)); err == nil {
				t.Fatalf("accepted task path %q", path)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "specs", "test")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(validDefinition())
	if err != nil {
		t.Fatal(err)
	}
	write("spec.md", []byte(markdown))
	write("tasks.json", data)
	if _, err := Load(root, "specs/test"); err != nil {
		t.Fatal(err)
	}
	write("tasks.json", []byte(string(data[:len(data)-1])+`,"unexpected":true}`))
	if _, err := Load(root, "specs/test"); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
	write("tasks.json", append(data, []byte(` {}`)...))
	if _, err := Load(root, "specs/test"); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := Load(root, "../test"); err == nil {
		t.Fatal("path escape accepted")
	}
}

func TestLoadRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "specs", "test"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(outside, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "specs", "test", "spec.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	data, err := json.Marshal(validDefinition())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "specs", "test", "tasks.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "specs/test"); err == nil {
		t.Fatal("symlink input accepted")
	}
}
