package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	"my-bilibili/internal/runlog"
	"my-bilibili/internal/spec"
)

const maxFindingsFileBytes = 1 << 20 // 1 MiB

type scopeResult struct {
	SpecID       string   `json:"spec_id"`
	TaskID       string   `json:"task_id"`
	Base         string   `json:"base"`
	Head         string   `json:"head"`
	OK           bool     `json:"ok"`
	CheckedPaths []string `json:"checked_paths"`
	Violations   []string `json:"violations"`
}

func verifyScopeCommand(ctx context.Context, root string, args []string, out, errOut io.Writer) error {
	flags := newFlags("verify-scope", errOut)
	specPath := flags.String("spec", "", "repository-relative spec directory")
	taskID := flags.String("task", "", "task ID declared in tasks.json")
	baseRev := flags.String("base", "HEAD", "base Git revision")
	headRev := flags.String("head", "", "optional head Git revision (compares base..head without working tree)")
	if err := parse(flags, args); err != nil {
		return err
	}
	if strings.TrimSpace(*specPath) == "" || strings.TrimSpace(*taskID) == "" {
		return errors.New("verify-scope requires --spec and --task")
	}
	if err := validateGitRevisionArg("base", *baseRev); err != nil {
		return err
	}
	if *headRev != "" {
		if err := validateGitRevisionArg("head", *headRev); err != nil {
			return err
		}
	}
	snapshot, err := spec.Load(root, *specPath)
	if err != nil {
		return err
	}
	resolvedBase, resolvedHead, changed, err := collectChangedPaths(ctx, root, *baseRev, *headRev)
	if err != nil {
		return err
	}
	violations, scopeErr := spec.CheckWriteScope(snapshot.Definition, *taskID, changed)
	if scopeErr != nil && len(violations) == 0 {
		return scopeErr
	}
	if changed == nil {
		changed = []string{}
	}
	if violations == nil {
		violations = []string{}
	}
	result := scopeResult{
		SpecID:       snapshot.Definition.SpecID,
		TaskID:       *taskID,
		Base:         resolvedBase,
		Head:         resolvedHead,
		OK:           len(violations) == 0,
		CheckedPaths: changed,
		Violations:   violations,
	}
	if err := printJSON(out, result); err != nil {
		return errors.Join(scopeErr, err)
	}
	return scopeErr
}

func validateGitRevisionArg(name, rev string) error {
	if rev == "" {
		return fmt.Errorf("--%s revision cannot be empty", name)
	}
	if strings.HasPrefix(rev, "-") {
		return fmt.Errorf("--%s revision %q cannot start with '-'", name, rev)
	}
	for _, r := range rev {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("--%s revision %q cannot contain whitespace or control characters", name, rev)
		}
	}
	return nil
}

func collectChangedPaths(ctx context.Context, root, baseRev, headRev string) (string, string, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gitOut := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		return cmd.Output()
	}

	resolveCommit := func(rev string) (string, bool, error) {
		data, err := gitOut("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return "", false, nil
			}
			return "", false, fmt.Errorf("resolve revision %q: %w", rev, err)
		}
		return strings.TrimSpace(string(data)), true, nil
	}

	seen := make(map[string]bool)
	var paths []string
	addNullSeparated := func(raw []byte) {
		for _, entry := range bytes.Split(raw, []byte{0}) {
			if len(entry) == 0 {
				continue
			}
			p := string(entry)
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}

	if headRev != "" {
		baseSHA, ok, err := resolveCommit(baseRev)
		if err != nil {
			return "", "", nil, err
		}
		if !ok {
			return "", "", nil, fmt.Errorf("base revision %q does not resolve to a commit", baseRev)
		}
		headSHA, ok, err := resolveCommit(headRev)
		if err != nil {
			return "", "", nil, err
		}
		if !ok {
			return "", "", nil, fmt.Errorf("head revision %q does not resolve to a commit", headRev)
		}
		raw, err := gitOut("diff", "--name-only", "--no-renames", "-z", baseSHA, headSHA, "--")
		if err != nil {
			return "", "", nil, fmt.Errorf("git diff %s..%s: %w", baseSHA, headSHA, err)
		}
		addNullSeparated(raw)
		sort.Strings(paths)
		return baseSHA, headSHA, paths, nil
	}

	baseSHA, ok, err := resolveCommit(baseRev)
	if err != nil {
		return "", "", nil, err
	}
	if !ok {
		if baseRev != "HEAD" {
			return "", "", nil, fmt.Errorf("base revision %q does not resolve to a commit", baseRev)
		}
		// Unborn HEAD: inspect staged index plus untracked non-ignored files.
		cached, err := gitOut("diff", "--cached", "--name-only", "--no-renames", "-z", "--")
		if err != nil {
			return "", "", nil, fmt.Errorf("git diff --cached: %w", err)
		}
		addNullSeparated(cached)
		untracked, err := gitOut("ls-files", "--others", "--exclude-standard", "-z")
		if err != nil {
			return "", "", nil, fmt.Errorf("git ls-files --others: %w", err)
		}
		addNullSeparated(untracked)
		sort.Strings(paths)
		return "unborn", "", paths, nil
	}

	// Resolved commit: union staged changes, working-tree changes, and untracked files.
	cached, err := gitOut("diff", "--cached", "--name-only", "--no-renames", "-z", baseSHA, "--")
	if err != nil {
		return "", "", nil, fmt.Errorf("git diff --cached %s: %w", baseSHA, err)
	}
	addNullSeparated(cached)
	worktree, err := gitOut("diff", "--name-only", "--no-renames", "-z", baseSHA, "--")
	if err != nil {
		return "", "", nil, fmt.Errorf("git diff %s: %w", baseSHA, err)
	}
	addNullSeparated(worktree)
	untracked, err := gitOut("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", "", nil, fmt.Errorf("git ls-files --others: %w", err)
	}
	addNullSeparated(untracked)
	sort.Strings(paths)
	return baseSHA, "", paths, nil
}

func loadFindingsFile(path string) ([]runlog.Finding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open findings file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat findings file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("findings file must be a regular file")
	}
	if info.Size() > maxFindingsFileBytes {
		return nil, fmt.Errorf("findings file exceeds %d bytes", maxFindingsFileBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFindingsFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read findings file: %w", err)
	}
	if len(data) > maxFindingsFileBytes {
		return nil, fmt.Errorf("findings file exceeds %d bytes", maxFindingsFileBytes)
	}
	var findings []runlog.Finding
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&findings); err != nil {
		return nil, fmt.Errorf("decode findings JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("findings file must contain exactly one JSON array")
	}
	if findings == nil {
		return nil, errors.New("findings file must contain a JSON array")
	}
	return findings, nil
}
