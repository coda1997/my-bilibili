package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"my-bilibili/internal/check"
	"my-bilibili/internal/runlog"
	"my-bilibili/internal/spec"
)

const usage = `Usage:
  dev check [--race] [--run <id>]
  dev run new --spec specs/<id> [--allow-dirty]
  dev run list
  dev run status --id <id> [--events]
  dev run note --id <id> --phase <phase> --message <text> [--evidence evidence/<file>]
  dev run finish --id <id> --outcome <outcome> --message <text> [--evidence evidence/<file>]

Phases: planning, implementation, validation, review, handoff.
Outcomes: succeeded, failed, needs-human. Outcomes are not review approvals.
Evidence flags may be repeated. Files must already exist in the run directory.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(usage)
		return
	}
	root, err := repositoryRoot()
	if err == nil {
		err = execute(ctx, root, args, os.Stdout, os.Stderr)
	}
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, root string, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	store, err := runlog.New(root)
	if err != nil {
		return err
	}
	switch args[0] {
	case "check":
		flags := newFlags("check", errOut)
		race := flags.Bool("race", false, "enable the Go race detector")
		id := flags.String("run", "", "record the check log in this open run")
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		if *id == "" {
			return check.Run(ctx, root, out, *race)
		}
		return recordedCheck(ctx, root, store, *id, *race, out, check.Run)
	case "run":
		return runCommand(root, store, args[1:], out, errOut)
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func runCommand(root string, store *runlog.Store, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	flags := newFlags("run "+args[0], errOut)
	switch args[0] {
	case "new":
		path := flags.String("spec", "", "repository-relative spec directory")
		allowDirty := flags.Bool("allow-dirty", false, "allow an unreproducible development baseline")
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		snapshot, err := spec.Load(root, *path)
		if err != nil {
			return err
		}
		source, err := sourceInfo(root)
		if err != nil {
			return err
		}
		if source.Dirty && !*allowDirty {
			return errors.New("source is dirty or unborn; commit a baseline or explicitly use --allow-dirty for development")
		}
		manifest, err := store.Start(snapshot, source)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, manifest.ID)
		return err
	case "list":
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		list, err := store.List()
		if err != nil {
			return err
		}
		return printJSON(out, list)
	case "status":
		id := flags.String("id", "", "run ID")
		events := flags.Bool("events", false, "include complete event history")
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		view, err := store.Read(*id)
		if err != nil {
			return err
		}
		if *events {
			return printJSON(out, view)
		}
		return printJSON(out, view.Summary())
	case "note", "finish":
		id := flags.String("id", "", "run ID")
		message := flags.String("message", "", "checkpoint or outcome description")
		var paths []string
		flags.Func("evidence", "existing file relative to this run", func(path string) error {
			paths = append(paths, path)
			return nil
		})
		var phase, outcome string
		if args[0] == "note" {
			flags.StringVar(&phase, "phase", "", "planning, implementation, validation, review, or handoff")
		} else {
			flags.StringVar(&outcome, "outcome", "", "succeeded, failed, or needs-human")
		}
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		source, err := sourceInfo(root)
		if err != nil {
			return err
		}
		if args[0] == "note" {
			err = store.Note(*id, phase, *message, paths, source)
		} else {
			err = store.Finish(*id, outcome, *message, paths, source)
		}
		if err != nil {
			return err
		}
		view, err := store.Read(*id)
		if err != nil {
			return err
		}
		return printJSON(out, view.Summary())
	default:
		return fmt.Errorf("unknown run command %q\n%s", args[0], usage)
	}
}

type checkRunner func(context.Context, string, io.Writer, bool) error

func recordedCheck(ctx context.Context, root string, store *runlog.Store, id string, race bool, out io.Writer, run checkRunner) error {
	source, err := sourceInfo(root)
	if err != nil {
		return err
	}
	file, relative, err := store.CreateEvidence(id, "check-*.log")
	if err != nil {
		return err
	}
	w := io.MultiWriter(out, file)
	_, headerErr := fmt.Fprintf(w, "Go: %s; platform: %s/%s; race: %t\nSource: %s; dirty: %t\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, race, source.Commit, source.Dirty)
	checkErr := headerErr
	if checkErr == nil {
		checkErr = run(ctx, root, w, race)
	}
	after, sourceErr := sourceInfo(root)
	if sourceErr != nil {
		checkErr = errors.Join(checkErr, sourceErr)
	} else if after != source {
		checkErr = errors.Join(checkErr, errors.New("source provenance changed during checks; rerun on a stable baseline"))
	}
	message := "Checks passed; independent review and acceptance are still required"
	if checkErr != nil {
		message = "Checks failed: " + checkErr.Error()
	}
	_, resultErr := fmt.Fprintln(w, message)
	fileErr := errors.Join(resultErr, file.Sync(), file.Close())
	if fileErr != nil {
		return errors.Join(checkErr, fileErr)
	}
	if sourceErr == nil {
		source = after
	}
	recordErr := store.Note(id, "validation", message, []string{relative}, source)
	return errors.Join(checkErr, recordErr)
}

func newFlags(name string, out io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(out)
	return flags
}

func parse(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	return nil
}

func printJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func repositoryRoot() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("run this command inside the Git checkout: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(data)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", errors.New("repository root has no go.mod")
	}
	return root, nil
}

func sourceInfo(root string) (runlog.Source, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		return cmd.Output()
	}
	data, err := command("rev-parse", "--verify", "--quiet", "HEAD")
	source := runlog.Source{Commit: strings.TrimSpace(string(data))}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return runlog.Source{}, fmt.Errorf("read source revision: %w", err)
		}
		source.Commit = "unborn"
	}
	status, err := command("status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return runlog.Source{}, fmt.Errorf("read source status: %w", err)
	}
	source.Dirty = len(status) != 0 || source.Commit == "unborn"
	return source, nil
}
