package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"my-bilibili/internal/previewbundle"
)

func main() {
	if err := command(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func command(args []string, out, stderr io.Writer) error {
	f := flag.NewFlagSet("package-preview", flag.ContinueOnError)
	f.SetOutput(stderr)
	source := f.String("source", ".", "repository source root")
	output := f.String("output", "", "fresh absolute developer .app output path")
	e2e := f.Bool("e2e", false, "include test-only automation")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *output == "" {
		return fmt.Errorf("supply -output with a fresh absolute .app path")
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("developer preview packaging requires macOS")
	}
	root, err := filepath.Abs(*source)
	if err != nil {
		return err
	}
	if err = previewbundle.VerifyRuntime(); err != nil {
		return err
	}
	if err = previewbundle.Populate(*output, filepath.Join(root, "internal", "fixture", "assets")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	buildArgs := []string{"build", "-trimpath", "-o", filepath.Join(*output, "Contents", "MacOS", "my-bilibili")}
	if *e2e {
		buildArgs = append(buildArgs, "-tags", "e2e")
	}
	buildArgs = append(buildArgs, "./cmd/my-bilibili")
	cmd := exec.CommandContext(ctx, "go", buildArgs...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, out, stderr
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("preview build failed; incomplete fresh output retained: %w", err)
	}
	h, err := previewbundle.Hash(filepath.Join(*output, "Contents", "MacOS", "my-bilibili"))
	if err == nil {
		_, err = fmt.Fprintln(out, "developer preview executable SHA256", h)
	}
	return err
}
