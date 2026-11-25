package runlog

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"my-bilibili/internal/spec"
)

type Source struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	SpecID        string    `json:"spec_id"`
	SpecPath      string    `json:"spec_path"`
	SpecSHA256    string    `json:"spec_sha256"`
	TasksSHA256   string    `json:"tasks_sha256"`
	Source        Source    `json:"source"`
}

type Evidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Event struct {
	Sequence         int            `json:"sequence"`
	At               time.Time      `json:"at"`
	Kind             string         `json:"kind"`
	Phase            string         `json:"phase,omitempty"`
	Outcome          string         `json:"outcome,omitempty"`
	Message          string         `json:"message"`
	Source           Source         `json:"source"`
	Evidence         []Evidence     `json:"evidence,omitempty"`
	TaskID           string         `json:"task_id,omitempty"`
	Role             string         `json:"role,omitempty"`
	ReviewRound      int            `json:"review_round,omitempty"`
	FixIteration     int            `json:"fix_iteration,omitempty"`
	FailureSignature string         `json:"failure_signature,omitempty"`
	DurationMS       int64          `json:"duration_ms,omitempty"`
	Findings         []Finding      `json:"findings,omitempty"`
	Override         *HumanOverride `json:"override,omitempty"`
}

type View struct {
	Manifest Manifest                `json:"manifest"`
	Status   string                  `json:"status"`
	Events   []Event                 `json:"events"`
	Budgets  map[string]BudgetStatus `json:"budgets,omitempty"`
}

type Summary struct {
	Manifest Manifest                `json:"manifest"`
	Status   string                  `json:"status"`
	Events   int                     `json:"event_count"`
	Latest   Event                   `json:"latest_event"`
	Budgets  map[string]BudgetStatus `json:"budgets,omitempty"`
}

func (view View) Summary() Summary {
	return Summary{
		Manifest: view.Manifest,
		Status:   view.Status,
		Events:   len(view.Events),
		Latest:   view.Events[len(view.Events)-1],
		Budgets:  view.Budgets,
	}
}

type Store struct {
	base string
}

var runID = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[a-f0-9]{12}$`)
var commitID = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
var eventName = regexp.MustCompile(`^[0-9]{6}\.json$`)

func New(root string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	return &Store{base: filepath.Join(root, ".agent", "runs")}, nil
}

func (s *Store) Start(snapshot spec.Snapshot, source Source) (Manifest, error) {
	if err := validSource(source); err != nil {
		return Manifest{}, err
	}
	decoded, err := decodeStrictTaskDefinition(snapshot.TaskJSON)
	if err != nil {
		return Manifest{}, err
	}
	if !definitionsEqual(decoded, snapshot.Definition) {
		return Manifest{}, errors.New("snapshot TaskJSON does not match Definition")
	}
	if err := spec.Validate(decoded, snapshot.Markdown); err != nil {
		return Manifest{}, err
	}
	if !spec.ValidPath(snapshot.Path) {
		return Manifest{}, errors.New("invalid snapshot path")
	}
	for _, dir := range []string{filepath.Dir(s.base), s.base} {
		if err := ensureDirectory(dir); err != nil {
			return Manifest{}, err
		}
	}
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Manifest{}, err
	}
	now := time.Now().UTC()
	manifest := Manifest{
		SchemaVersion: 2,
		ID:            now.Format("20060102T150405Z") + "-" + hex.EncodeToString(random[:]),
		CreatedAt:     now,
		SpecID:        snapshot.Definition.SpecID,
		SpecPath:      snapshot.Path,
		SpecSHA256:    digest(snapshot.Markdown),
		TasksSHA256:   digest(snapshot.TaskJSON),
		Source:        source,
	}
	temp, err := os.MkdirTemp(s.base, ".creating-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(temp)
	for _, child := range []string{"inputs", "events", "evidence"} {
		if err := os.Mkdir(filepath.Join(temp, child), 0o700); err != nil {
			return Manifest{}, err
		}
	}
	for path, data := range map[string][]byte{"inputs/spec.md": snapshot.Markdown, "inputs/tasks.json": snapshot.TaskJSON} {
		if err := writeNew(filepath.Join(temp, filepath.FromSlash(path)), data); err != nil {
			return Manifest{}, err
		}
	}
	if err := writeJSON(filepath.Join(temp, "manifest.json"), manifest); err != nil {
		return Manifest{}, err
	}
	event := Event{Sequence: 1, At: now, Kind: "started", Message: "Run started", Source: source}
	if err := writeJSON(filepath.Join(temp, "events", "000001.json"), event); err != nil {
		return Manifest{}, err
	}
	if _, err := os.Lstat(filepath.Join(s.base, manifest.ID)); !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, errors.New("run ID collision or inaccessible destination")
	}
	if err := os.Rename(temp, filepath.Join(s.base, manifest.ID)); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (s *Store) Read(id string) (View, error) {
	view, _, err := s.readWithTracker(id)
	return view, err
}

func (s *Store) readWithTracker(id string) (View, *budgetTracker, error) {
	var view View
	dir, err := s.directory(id)
	if err != nil {
		return view, nil, err
	}
	if err := readJSON(dir, "manifest.json", &view.Manifest); err != nil {
		return View{}, nil, err
	}
	m := view.Manifest
	if (m.SchemaVersion != 1 && m.SchemaVersion != 2) || m.ID != id || m.CreatedAt.IsZero() || m.SpecID == "" || !spec.ValidPath(m.SpecPath) {
		return View{}, nil, errors.New("invalid run manifest")
	}
	if err := validSource(m.Source); err != nil {
		return View{}, nil, err
	}
	frozenData := make(map[string][]byte, 2)
	for path, want := range map[string]string{"inputs/spec.md": m.SpecSHA256, "inputs/tasks.json": m.TasksSHA256} {
		file, err := fileDigest(dir, path)
		if err != nil {
			return View{}, nil, err
		}
		if file.SHA256 != want {
			return View{}, nil, fmt.Errorf("frozen input digest mismatch: %s", path)
		}
		filename, err := checkedPath(dir, path)
		if err != nil {
			return View{}, nil, err
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return View{}, nil, err
		}
		if digest(data) != want {
			return View{}, nil, fmt.Errorf("frozen input digest mismatch: %s", path)
		}
		frozenData[path] = data
	}
	def, err := decodeStrictTaskDefinition(frozenData["inputs/tasks.json"])
	if err != nil {
		return View{}, nil, err
	}
	if def.SpecID != m.SpecID {
		return View{}, nil, errors.New("frozen tasks.json spec_id does not match manifest")
	}
	if err := spec.Validate(def, frozenData["inputs/spec.md"]); err != nil {
		return View{}, nil, fmt.Errorf("frozen spec inputs invalid: %w", err)
	}
	validTasks := make(map[string]bool, len(def.Tasks))
	for _, task := range def.Tasks {
		validTasks[task.ID] = true
	}
	tracker := newBudgetTracker(m.SchemaVersion, validTasks)

	eventDir, err := checkedPath(dir, "events")
	if err != nil {
		return View{}, nil, err
	}
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		return View{}, nil, err
	}
	view.Status = "running"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pending-") {
			continue
		}
		if !eventName.MatchString(entry.Name()) || entry.IsDir() {
			return View{}, nil, fmt.Errorf("unexpected event entry %s", entry.Name())
		}
		var event Event
		if err := readJSON(dir, "events/"+entry.Name(), &event); err != nil {
			return View{}, nil, err
		}
		expected := len(view.Events) + 1
		if event.Sequence != expected || entry.Name() != fmt.Sprintf("%06d.json", expected) {
			return View{}, nil, errors.New("event sequence gap or mismatch")
		}
		if view.Status != "running" {
			return View{}, nil, errors.New("event appears after run was closed")
		}
		if err := validEvent(event); err != nil {
			return View{}, nil, err
		}
		if err := tracker.apply(event); err != nil {
			return View{}, nil, err
		}
		if expected == 1 && (event.Source != m.Source || !event.At.Equal(m.CreatedAt)) {
			return View{}, nil, errors.New("started event does not match manifest provenance")
		}
		for _, evidence := range event.Evidence {
			actual, err := collectEvidence(dir, evidence.Path)
			if err != nil {
				return View{}, nil, err
			}
			if actual != evidence {
				return View{}, nil, fmt.Errorf("evidence digest or size mismatch: %s", evidence.Path)
			}
		}
		if event.Kind == "finished" {
			if event.Outcome == "succeeded" && !hasEvidence(view.Events) && len(event.Evidence) == 0 {
				return View{}, nil, errors.New("successful outcome has no evidence")
			}
			view.Status = event.Outcome
		}
		view.Events = append(view.Events, event)
	}
	if len(view.Events) == 0 {
		return View{}, nil, errors.New("run has no started event")
	}
	view.Budgets = tracker.budgets()
	return view, tracker, nil
}

func (s *Store) List() ([]Summary, error) {
	result := make([]Summary, 0)
	if _, err := os.Lstat(s.base); errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if _, err := checkedPath(filepath.Dir(filepath.Dir(s.base)), ".agent/runs"); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.base)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".creating-") {
			continue
		}
		view, err := s.Read(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", entry.Name(), err)
		}
		result = append(result, view.Summary())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Manifest.ID < result[j].Manifest.ID })
	return result, nil
}

func (s *Store) Note(id, phase, message string, paths []string, source Source) error {
	return s.NoteWithOptions(id, NoteOptions{Phase: phase, Message: message, Evidence: paths}, source)
}

func (s *Store) NoteWithOptions(id string, opts NoteOptions, source Source) error {
	return s.append(id, Event{
		Kind:             "checkpoint",
		Phase:            opts.Phase,
		Message:          opts.Message,
		Source:           source,
		TaskID:           opts.TaskID,
		Role:             opts.Role,
		ReviewRound:      opts.ReviewRound,
		FixIteration:     opts.FixIteration,
		FailureSignature: opts.FailureSignature,
		DurationMS:       opts.DurationMS,
		Findings:         opts.Findings,
	}, opts.Evidence)
}

func (s *Store) HumanOverride(id, taskID, message string, override HumanOverride, paths []string, source Source) error {
	o := override
	return s.append(id, Event{
		Kind:     "human-override",
		Message:  message,
		Source:   source,
		TaskID:   taskID,
		Override: &o,
	}, paths)
}

func (s *Store) Finish(id, outcome, message string, paths []string, source Source) error {
	return s.append(id, Event{Kind: "finished", Outcome: outcome, Message: message, Source: source}, paths)
}

func (s *Store) CreateEvidence(id, pattern string) (*os.File, string, error) {
	view, err := s.Read(id)
	if err != nil {
		return nil, "", err
	}
	if view.Status != "running" {
		return nil, "", errors.New("run is already closed")
	}
	dir, err := s.directory(id)
	if err != nil {
		return nil, "", err
	}
	evidenceDir, err := checkedPath(dir, "evidence")
	if err != nil {
		return nil, "", err
	}
	file, err := os.CreateTemp(evidenceDir, pattern)
	if err != nil {
		return nil, "", err
	}
	return file, "evidence/" + filepath.Base(file.Name()), nil
}

func (s *Store) append(id string, event Event, paths []string) error {
	dir, err := s.directory(id)
	if err != nil {
		return err
	}
	lock := filepath.Join(dir, ".write-lock")
	if err := os.Mkdir(lock, 0o700); err != nil {
		return fmt.Errorf("cannot acquire writer lock (inspect %s before recovering a stale lock): %w", lock, err)
	}
	defer os.RemoveAll(lock)
	if err := writeJSON(filepath.Join(lock, "owner.json"), struct {
		PID int       `json:"pid"`
		At  time.Time `json:"at"`
	}{os.Getpid(), time.Now().UTC()}); err != nil {
		return err
	}
	view, tracker, err := s.readWithTracker(id)
	if err != nil {
		return err
	}
	if view.Status != "running" {
		return errors.New("run is already closed")
	}
	event.Sequence, event.At = len(view.Events)+1, time.Now().UTC()
	if event.Sequence > 999999 {
		return errors.New("event limit reached; start a continuation run")
	}
	if err := validEvent(event); err != nil {
		return err
	}
	if err := tracker.apply(event); err != nil {
		return err
	}
	for _, path := range paths {
		evidence, err := collectEvidence(dir, path)
		if err != nil {
			return err
		}
		event.Evidence = append(event.Evidence, evidence)
	}
	if event.Outcome == "succeeded" && !hasEvidence(view.Events) && len(event.Evidence) == 0 {
		return errors.New("successful outcome requires recorded evidence; this is still not review approval")
	}
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}
	// Publish one immutable event, rather than updating a log and a second state file.
	file, err := os.CreateTemp(filepath.Join(dir, "events"), ".pending-")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	destination := filepath.Join(dir, "events", fmt.Sprintf("%06d.json", event.Sequence))
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("event destination already exists or is inaccessible")
	}
	return os.Rename(temp, destination)
}

func (s *Store) directory(id string) (string, error) {
	if !runID.MatchString(id) {
		return "", fmt.Errorf("invalid run ID %q", id)
	}
	root := filepath.Dir(filepath.Dir(s.base))
	return checkedPath(root, ".agent/runs/"+id)
}

func validSource(source Source) error {
	if source.Commit == "unborn" && source.Dirty || commitID.MatchString(source.Commit) {
		return nil
	}
	return errors.New("invalid source provenance")
}

func validEvent(event Event) error {
	if event.At.IsZero() || strings.TrimSpace(event.Message) == "" {
		return errors.New("event requires a timestamp and message")
	}
	if err := validSource(event.Source); err != nil {
		return err
	}
	if event.Sequence == 1 {
		if event.Kind != "started" || event.Phase != "" || event.Outcome != "" || len(event.Evidence) != 0 {
			return errors.New("first event must be a plain started event")
		}
		return nil
	}
	switch event.Kind {
	case "checkpoint":
		if event.Outcome != "" {
			return errors.New("checkpoint cannot set an outcome")
		}
		switch event.Phase {
		case "planning", "implementation", "validation", "review", "handoff":
			return nil
		default:
			return fmt.Errorf("invalid checkpoint phase %q", event.Phase)
		}
	case "finished":
		if event.Phase != "" {
			return errors.New("finished event cannot set a phase")
		}
		switch event.Outcome {
		case "succeeded", "failed", "needs-human":
			return nil
		default:
			return fmt.Errorf("invalid outcome %q", event.Outcome)
		}
	case "human-override":
		if event.Phase != "" || event.Outcome != "" {
			return errors.New("human-override event cannot set phase or outcome")
		}
		if event.Override == nil {
			return errors.New("human-override event requires override payload")
		}
		return nil
	default:
		return fmt.Errorf("invalid event kind %q", event.Kind)
	}
}

func hasEvidence(events []Event) bool {
	for _, event := range events {
		if len(event.Evidence) != 0 {
			return true
		}
	}
	return false
}

func ensureDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("not a plain directory: %s", path)
	}
	return nil
}

func checkedPath(root, path string) (string, error) {
	if !spec.ValidPath(path) {
		return "", fmt.Errorf("unsafe relative path %q", path)
	}
	filename := filepath.Join(root, filepath.FromSlash(path))
	resolved, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return "", err
	}
	if resolved != filename {
		return "", fmt.Errorf("symlinks are not allowed: %s", filename)
	}
	return filename, nil
}

func collectEvidence(dir, path string) (Evidence, error) {
	if !strings.HasPrefix(path, "evidence/") {
		return Evidence{}, errors.New("evidence must be a file below this run's evidence/ directory")
	}
	return fileDigest(dir, path)
}

func fileDigest(dir, path string) (Evidence, error) {
	filename, err := checkedPath(dir, path)
	if err != nil {
		return Evidence{}, err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return Evidence{}, err
	}
	if !info.Mode().IsRegular() {
		return Evidence{}, fmt.Errorf("not a regular evidence/input file: %s", path)
	}
	file, err := os.Open(filename)
	if err != nil {
		return Evidence{}, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return Evidence{}, err
	}
	if !info.Mode().IsRegular() {
		return Evidence{}, fmt.Errorf("not a regular evidence/input file: %s", path)
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{Path: path, SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: n}, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeNew(path, append(data, '\n'))
}

func readJSON(dir, path string, target any) error {
	filename, err := checkedPath(dir, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular JSON record: %s", path)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON in %s", path)
	}
	return nil
}
