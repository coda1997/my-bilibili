package check

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"my-bilibili/internal/spec"
)

func Run(ctx context.Context, root string, out io.Writer, race bool) error {
	testArgs := []string{"test", "-count=1"}
	if race {
		testArgs = append(testArgs, "-race")
	}
	testArgs = append(testArgs, "./...")
	command := func(args ...string) func() error {
		return func() error {
			cmd := exec.CommandContext(ctx, "go", args...)
			cmd.Dir, cmd.Stdout, cmd.Stderr = root, out, out
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
			return cmd.Run()
		}
	}
	steps := []struct {
		name string
		run  func() error
	}{
		{"spec contracts", func() error { return Specs(root) }},
		{"agent workflow integrity", func() error {
			count, err := Workflow(root)
			if err == nil {
				_, err = fmt.Fprintf(out, "workflow files checked: %d\n", count)
			}
			return err
		}},
		{"Go formatting (read-only)", func() error { return Formatting(root) }},
		{"go mod tidy -diff", command("mod", "tidy", "-diff")},
		{"go vet ./...", command("vet", "./...")},
		{"go " + strings.Join(testArgs, " "), command(testArgs...)},
		{"go build ./...", command("build", "./...")},
	}
	for i, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "[%d/%d] %s\n", i+1, len(steps), step.name); err != nil {
			return err
		}
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	_, err := fmt.Fprintln(out, "All checks passed. This is not independent review or release approval.")
	return err
}

var grandfatheredSpecNumericPrefixes = map[string]map[string]bool{
	"0004": {
		"0004-client-p0":               true,
		"0004-workflow-clarifications": true,
	},
	"0025": {
		"0025-agent-workflow-integrity": true,
		"0025-p0-async-source":          true,
	},
}

var grandfatheredSpecACPrefixes = map[string]map[string]bool{
	"M2": {
		"0015-m2-fixture-preview":     true,
		"0016-m2-policy-continuation": true,
	},
}

func Specs(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "specs"))
	if err != nil {
		return err
	}
	count := 0
	byNumericPrefix := make(map[string][]string)
	byACPrefix := make(map[string][]string)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked spec directory: %s", entry.Name())
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		snapshot, err := spec.Load(root, "specs/"+name)
		if err != nil {
			return err
		}
		if prefix, ok := specNumericPrefix(name); ok {
			byNumericPrefix[prefix] = append(byNumericPrefix[prefix], name)
		}
		seenACPrefixes := make(map[string]bool)
		for _, task := range snapshot.Definition.Tasks {
			for _, acceptanceID := range task.Acceptance {
				if prefix, _, ok := strings.Cut(acceptanceID, "-AC-"); ok && !seenACPrefixes[prefix] {
					seenACPrefixes[prefix] = true
					byACPrefix[prefix] = append(byACPrefix[prefix], snapshot.Definition.SpecID)
				}
			}
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no specs found")
	}
	if err := validateUniqueSpecGroups("spec numeric prefix", byNumericPrefix, grandfatheredSpecNumericPrefixes); err != nil {
		return err
	}
	if err := validateUniqueSpecGroups("acceptance prefix", byACPrefix, grandfatheredSpecACPrefixes); err != nil {
		return err
	}
	return nil
}

func specNumericPrefix(name string) (string, bool) {
	prefix, _, ok := strings.Cut(name, "-")
	if !ok || prefix == "" {
		return "", false
	}
	for i := 0; i < len(prefix); i++ {
		if prefix[i] < '0' || prefix[i] > '9' {
			return "", false
		}
	}
	return prefix, true
}

func validateUniqueSpecGroups(kind string, groups map[string][]string, grandfathered map[string]map[string]bool) error {
	keys := make([]string, 0, len(groups))
	for prefix := range groups {
		keys = append(keys, prefix)
	}
	sort.Strings(keys)
	for _, prefix := range keys {
		names := groups[prefix]
		if len(names) < 2 {
			continue
		}
		sort.Strings(names)
		allowed := grandfathered[prefix]
		permitted := len(allowed) > 0
		if permitted {
			for _, name := range names {
				if !allowed[name] {
					permitted = false
					break
				}
			}
		}
		if !permitted {
			return fmt.Errorf("duplicate %s %q shared by: %s", kind, prefix, strings.Join(names, ", "))
		}
	}
	return nil
}

func Formatting(root string) error {
	var unformatted []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".agent", "vendor", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked Go source is not supported: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted, err := format.Source(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if !bytes.Equal(data, formatted) {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			unformatted = append(unformatted, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(unformatted) != 0 {
		return fmt.Errorf("run gofmt on: %s", strings.Join(unformatted, ", "))
	}
	return nil
}
