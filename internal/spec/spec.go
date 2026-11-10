package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type Task struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	DependsOn  []string `json:"depends_on"`
	WritePaths []string `json:"write_paths"`
	Acceptance []string `json:"acceptance"`
	Checks     []string `json:"checks"`
}

type Definition struct {
	SchemaVersion int    `json:"schema_version"`
	SpecID        string `json:"spec_id"`
	Tasks         []Task `json:"tasks"`
}

type Snapshot struct {
	Path       string
	Markdown   []byte
	TaskJSON   []byte
	Definition Definition
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var acceptanceHeading = regexp.MustCompile(`(?m)^### ([A-Z][A-Z0-9]*-AC-[0-9]+)[ \t]*\r?$`)
var windowsDevice = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)`)

// ValidPath uses portable, literal repository paths, not glob expressions.
func ValidPath(path string) bool {
	if path == "." || !fs.ValidPath(path) || strings.ContainsAny(path, `\:*?[]<>|"`) {
		return false
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, component := range strings.Split(path, "/") {
		if strings.TrimSpace(component) != component || strings.HasSuffix(component, ".") || windowsDevice.MatchString(component) {
			return false
		}
	}
	return true
}

func Load(root, path string) (Snapshot, error) {
	var snapshot Snapshot
	if !ValidPath(path) {
		return snapshot, fmt.Errorf("invalid spec directory %q", path)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return snapshot, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return snapshot, err
	}
	dir := filepath.Join(root, filepath.FromSlash(path))
	snapshot.Path = path
	for name, dest := range map[string]*[]byte{"spec.md": &snapshot.Markdown, "tasks.json": &snapshot.TaskJSON} {
		filename := filepath.Join(dir, name)
		resolved, err := filepath.EvalSymlinks(filename)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read %s: %w", filename, err)
		}
		if resolved != filename {
			return Snapshot{}, fmt.Errorf("symlinks are not allowed in spec inputs: %s", filename)
		}
		info, err := os.Stat(filename)
		if err != nil {
			return Snapshot{}, err
		}
		if !info.Mode().IsRegular() {
			return Snapshot{}, fmt.Errorf("not a regular input file: %s", filename)
		}
		*dest, err = os.ReadFile(filename)
		if err != nil {
			return Snapshot{}, err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(snapshot.TaskJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot.Definition); err != nil {
		return Snapshot{}, fmt.Errorf("tasks.json: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("tasks.json must contain exactly one JSON object")
	}
	if snapshot.Definition.SpecID != filepath.Base(dir) {
		return Snapshot{}, errors.New("spec_id must match its directory name")
	}
	if err := Validate(snapshot.Definition, snapshot.Markdown); err != nil {
		return Snapshot{}, fmt.Errorf("spec %s: %w", path, err)
	}
	return snapshot, nil
}

func Validate(def Definition, markdown []byte) error {
	if def.SchemaVersion != 1 || !identifier.MatchString(def.SpecID) || len(def.Tasks) == 0 {
		return errors.New("schema_version must be 1, with a valid spec_id and nonempty tasks")
	}
	criteria := make(map[string]bool)
	for _, match := range acceptanceHeading.FindAllSubmatch(markdown, -1) {
		id := string(match[1])
		if _, exists := criteria[id]; exists {
			return fmt.Errorf("duplicate acceptance heading %s", id)
		}
		criteria[id] = false
	}
	if len(criteria) == 0 {
		return errors.New("spec has no acceptance headings such as '### FND-AC-01'")
	}
	tasks := make(map[string]Task, len(def.Tasks))
	for _, task := range def.Tasks {
		if !identifier.MatchString(task.ID) || strings.TrimSpace(task.Title) == "" {
			return errors.New("every task requires a valid id and nonempty title")
		}
		if _, exists := tasks[task.ID]; exists {
			return fmt.Errorf("duplicate task %s", task.ID)
		}
		if len(task.WritePaths) == 0 || len(task.Acceptance) == 0 || len(task.Checks) == 0 {
			return fmt.Errorf("task %s requires write_paths, acceptance, and checks", task.ID)
		}
		for _, path := range task.WritePaths {
			if !ValidPath(path) {
				return fmt.Errorf("task %s has unsafe write path %q", task.ID, path)
			}
			for _, r := range path {
				if r > unicode.MaxASCII {
					return fmt.Errorf("task %s write path %q must use ASCII", task.ID, path)
				}
			}
		}
		for _, check := range task.Checks {
			if strings.TrimSpace(check) == "" {
				return fmt.Errorf("task %s has an empty check", task.ID)
			}
		}
		for _, id := range task.Acceptance {
			if _, exists := criteria[id]; !exists {
				return fmt.Errorf("task %s references unknown acceptance %s", task.ID, id)
			}
			criteria[id] = true
		}
		tasks[task.ID] = task
	}
	for id, covered := range criteria {
		if !covered {
			return fmt.Errorf("acceptance %s has no task", id)
		}
	}
	for _, task := range def.Tasks {
		seen := make(map[string]bool)
		for _, dep := range task.DependsOn {
			if _, exists := tasks[dep]; !exists {
				return fmt.Errorf("task %s depends on unknown task %s", task.ID, dep)
			}
			if seen[dep] {
				return fmt.Errorf("task %s has duplicate dependency %s", task.ID, dep)
			}
			seen[dep] = true
		}
	}
	visiting := make(map[string]bool)
	ancestors := make(map[string]map[string]bool)
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("task graph has a cycle at %s", id)
		}
		if _, done := ancestors[id]; done {
			return nil
		}
		visiting[id] = true
		deps := make(map[string]bool)
		for _, dep := range tasks[id].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
			deps[dep] = true
			for ancestor := range ancestors[dep] {
				deps[ancestor] = true
			}
		}
		visiting[id] = false
		ancestors[id] = deps
		return nil
	}
	for _, task := range def.Tasks {
		if err := visit(task.ID); err != nil {
			return err
		}
	}
	for i, left := range def.Tasks {
		for _, right := range def.Tasks[i+1:] {
			if ancestors[left.ID][right.ID] || ancestors[right.ID][left.ID] {
				continue
			}
			for _, a := range left.WritePaths {
				for _, b := range right.WritePaths {
					a, b := strings.ToLower(a), strings.ToLower(b)
					if a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
						return fmt.Errorf("unordered tasks %s and %s overlap at %s / %s", left.ID, right.ID, a, b)
					}
				}
			}
		}
	}
	return nil
}
