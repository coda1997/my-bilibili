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

func Specs(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "specs"))
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinked spec directory: %s", entry.Name())
		}
		if !entry.IsDir() {
			continue
		}
		if _, err := spec.Load(root, "specs/"+entry.Name()); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no specs found")
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
