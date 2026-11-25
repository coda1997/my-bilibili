package spec

import (
	"slices"
	"strings"
	"testing"
)

func TestCheckWriteScope(t *testing.T) {
	def := Definition{
		SchemaVersion: 1,
		SpecID:        "test",
		Tasks: []Task{
			{
				ID:         "T1",
				Title:      "Scoped task",
				WritePaths: []string{"internal/spec", "README.md"},
				Acceptance: []string{"TEST-AC-01"},
				Checks:     []string{"go test ./..."},
			},
		},
	}

	t.Run("accepts exact files and directory subtrees", func(t *testing.T) {
		violations, err := CheckWriteScope(def, "T1", []string{
			"README.md",
			"internal/spec/scope.go",
			"internal/spec/sub/nested.go",
		})
		if err != nil || len(violations) != 0 {
			t.Fatalf("got violations=%v err=%v, want pass", violations, err)
		}
	})

	t.Run("accepts empty changed paths", func(t *testing.T) {
		violations, err := CheckWriteScope(def, "T1", nil)
		if err != nil || len(violations) != 0 {
			t.Fatalf("got violations=%v err=%v, want pass", violations, err)
		}
	})

	t.Run("rejects out of scope paths sorted and deduplicated", func(t *testing.T) {
		violations, err := CheckWriteScope(def, "T1", []string{
			"internal/spec-other/file.go",
			"cmd/dev/main.go",
			"internal/spec/scope.go",
			"cmd/dev/main.go",
			"readme.md",
		})
		want := []string{"cmd/dev/main.go", "internal/spec-other/file.go", "readme.md"}
		if err == nil || !slices.Equal(violations, want) {
			t.Fatalf("got violations=%v err=%v, want %v with error", violations, err, want)
		}
	})

	t.Run("rejects unknown task", func(t *testing.T) {
		if _, err := CheckWriteScope(def, "MISSING", []string{"README.md"}); err == nil || !strings.Contains(err.Error(), "unknown task") {
			t.Fatalf("want unknown task error, got %v", err)
		}
	})

	t.Run("rejects unsafe changed paths", func(t *testing.T) {
		for _, bad := range []string{"", ".", "../outside.go", "/etc/passwd", `internal\spec\a.go`} {
			if _, err := CheckWriteScope(def, "T1", []string{bad}); err == nil {
				t.Errorf("accepted unsafe changed path %q", bad)
			}
		}
	})

	t.Run("rejects unvalidated definition with empty or unsafe write_paths", func(t *testing.T) {
		emptyDef := Definition{SchemaVersion: 1, SpecID: "test", Tasks: []Task{{ID: "T1"}}}
		if _, err := CheckWriteScope(emptyDef, "T1", nil); err == nil {
			t.Fatal("accepted task with empty write_paths")
		}
		badPathDef := Definition{SchemaVersion: 1, SpecID: "test", Tasks: []Task{{ID: "T1", WritePaths: []string{"../escape"}}}}
		if _, err := CheckWriteScope(badPathDef, "T1", nil); err == nil {
			t.Fatal("accepted task with unsafe write_path")
		}
		unicodeDef := Definition{SchemaVersion: 1, SpecID: "test", Tasks: []Task{{ID: "T1", WritePaths: []string{"internal/caf\u00e9"}}}}
		if _, err := CheckWriteScope(unicodeDef, "T1", nil); err == nil {
			t.Fatal("accepted task with non-ASCII write_path")
		}
	})
}
