// Package localhistory persists bounded device-local history under one
// filesystem owner. Construction does no I/O; Acquire starts its lifetime.
package localhistory

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"my-bilibili/internal/history"
)

const fileLimit = 1 << 20

// fileClose reports whether the Go handle was retired. A retired handle with
// an error cannot safely be retried: its numeric descriptor may be reused.
type fileClose func(*os.File) (retired bool, err error)

type fileOps struct {
	configDir func() (string, error)
	open      func(string, int, os.FileMode) (*os.File, error)
	create    func(string, string) (*os.File, error)
	read      func(io.Reader) ([]byte, error)
	write     func(*os.File, []byte) (int, error)
	sync      func(*os.File) error
	close     fileClose
	rename    func(string, string) error
	remove    func(string) error
	lock      func(*os.File) error
	unlock    func(*os.File) error
}

type ownedFile struct {
	file   *os.File
	remove string
}

// Store is transferred to one history.Manager and is not used concurrently.
// The OS lock, not this Go ownership convention, excludes other instances.
type Store struct {
	directory string
	canonical string
	dataPath  string
	lockFile  *os.File
	locked    bool
	terminal  bool
	closed    bool
	uncertain bool
	pending   []*ownedFile
	ops       fileOps
}

var _ history.Store = (*Store)(nil)

func New(directory string) (*Store, error) {
	if directory != "" && (!filepath.IsAbs(directory) || strings.ContainsRune(directory, 0)) {
		return nil, history.Invalid
	}
	return &Store{directory: directory, ops: fileOps{
		configDir: os.UserConfigDir, open: openNoFollow, create: os.CreateTemp, read: io.ReadAll,
		write: (*os.File).Write, sync: (*os.File).Sync,
		close:  func(file *os.File) (bool, error) { return true, file.Close() },
		rename: os.Rename, remove: os.Remove, lock: acquireLock, unlock: releaseLock,
	}}, nil
}

func (s *Store) Acquire() error {
	if s.terminal {
		return history.Closed
	}
	if !lockSupported {
		return history.LockUnsupported
	}
	if s.uncertain {
		return history.Storage
	}
	if s.locked {
		return nil
	}
	// A pre-close failure may leave an unowned handle awaiting explicit retry.
	if err := s.closeLock(); err != nil {
		return err
	}
	directory := s.directory
	if directory == "" {
		base, err := s.ops.configDir()
		if err != nil || !filepath.IsAbs(base) {
			return history.Storage
		}
		directory = filepath.Join(base, "my-bilibili")
	}
	directory = filepath.Clean(directory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return history.Storage
	}
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return history.Storage
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return history.Storage
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return history.Storage
	}
	s.canonical, s.dataPath = canonical, filepath.Join(canonical, "history.json")
	file, err := s.ops.open(filepath.Join(canonical, "history.lock"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return history.Storage
	}
	s.lockFile = file
	info, err = file.Stat()
	if err == nil && info.Mode().IsRegular() && info.Mode().Perm() == 0600 {
		err = s.ops.lock(file)
		if err == nil {
			s.locked = true
			return nil
		}
	} else {
		err = history.Storage
	}
	if cleanup := s.closeLock(); cleanup != nil {
		return cleanup
	}
	if errors.Is(err, history.InUse) {
		return history.InUse
	}
	return history.Storage
}

func (s *Store) owned() error {
	if s.terminal {
		return history.Closed
	}
	if !s.locked || s.uncertain {
		return history.Storage
	}
	return s.cleanup()
}

func (s *Store) Load() (records []history.Record, err error) {
	if err = s.owned(); err != nil {
		return nil, err
	}
	file, err := s.ops.open(s.dataPath, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return []history.Record{}, nil
	}
	if err != nil {
		return nil, history.Storage
	}
	s.pending = append(s.pending, &ownedFile{file: file})
	defer func() {
		if s.cleanup() != nil {
			records, err = nil, history.Storage
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, history.Storage
	}
	if info.Size() > fileLimit {
		return nil, history.Corrupt
	}
	data, err := s.ops.read(io.LimitReader(file, fileLimit+1))
	if err != nil {
		return nil, history.Storage
	}
	if len(data) > fileLimit {
		return nil, history.Corrupt
	}
	return decode(data)
}

func (s *Store) Save(records []history.Record) (err error) {
	if err = s.owned(); err != nil {
		return err
	}
	data, err := encode(records)
	if err != nil {
		return err
	}
	info, err := os.Lstat(s.dataPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) || err == nil &&
		(!info.Mode().IsRegular() || info.Mode().Perm() != 0600) {
		return history.Storage
	}
	file, err := s.ops.create(s.canonical, ".history-*")
	if err != nil {
		return history.Storage
	}
	temporary := &ownedFile{file: file, remove: file.Name()}
	s.pending = append(s.pending, temporary)
	replaced := false
	defer func() {
		if s.cleanup() != nil {
			err = history.Storage
		}
		if err != nil && replaced {
			err = history.Uncertain
		}
	}()
	if n, writeErr := s.ops.write(file, data); writeErr != nil || n != len(data) {
		return history.Storage
	}
	if s.ops.sync(file) != nil {
		return history.Storage
	}
	if s.closeFile(temporary) != nil {
		return history.Storage
	}
	if s.ops.rename(temporary.remove, s.dataPath) != nil {
		return history.Storage
	}
	temporary.remove = ""
	replaced = true
	directory, err := s.ops.open(s.canonical, os.O_RDONLY, 0)
	if err != nil {
		return history.Uncertain
	}
	s.pending = append(s.pending, &ownedFile{file: directory})
	if s.ops.sync(directory) != nil {
		return history.Uncertain
	}
	return nil
}

func (s *Store) closeFile(owned *ownedFile) error {
	if owned.file == nil {
		return nil
	}
	retired, err := s.ops.close(owned.file)
	if retired {
		owned.file = nil
	}
	if err != nil {
		// os.File.Close retires its Go handle even on error. Preserve that
		// uncertainty; never close an fd number which may now name another file.
		s.uncertain = s.uncertain || retired
		return history.Storage
	}
	return nil
}

func (s *Store) cleanup() error {
	failed := s.uncertain
	remaining := s.pending[:0]
	for _, owned := range s.pending {
		if s.closeFile(owned) != nil {
			failed = true
		}
		if owned.remove != "" {
			err := s.ops.remove(owned.remove)
			if err == nil || errors.Is(err, os.ErrNotExist) {
				owned.remove = ""
			} else {
				failed = true
			}
		}
		if owned.file != nil || owned.remove != "" {
			remaining = append(remaining, owned)
		}
	}
	clear(s.pending[len(remaining):])
	s.pending = remaining
	if failed {
		return history.Storage
	}
	return nil
}

func (s *Store) closeLock() error {
	if s.lockFile == nil {
		return nil
	}
	owned := &ownedFile{file: s.lockFile}
	err := s.closeFile(owned)
	s.lockFile = owned.file
	return err
}

// Close is terminal even if cleanup fails. Retry only the unfinished cleanup;
// never reopen or write a retired Store. A retired-but-uncertain descriptor
// cannot be proven clean by retry and continues to report Storage.
func (s *Store) Close() error {
	s.terminal = true
	if s.closed {
		return nil
	}
	if s.cleanup() != nil {
		return history.Storage
	}
	if s.locked {
		if s.ops.unlock(s.lockFile) != nil {
			return history.Storage
		}
		s.locked = false
	}
	if s.closeLock() != nil {
		return history.Storage
	}
	s.closed = true
	return nil
}
