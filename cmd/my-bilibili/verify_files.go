package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"my-bilibili/internal/history"
)

func verificationDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode().Perm() == 0700
}

func verificationOpen(path string, limit int64, private bool) (*os.File, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit || private && info.Mode().Perm() != 0600 {
		return nil, nil, history.Storage
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Size() != info.Size() {
		_ = file.Close()
		return nil, nil, history.Storage
	}
	return file, opened, nil
}

func verificationRead(path string, limit int64, private bool) ([]byte, error) {
	file, before, err := verificationOpen(path, limit, private)
	if err != nil {
		return nil, history.Storage
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil || statErr != nil || closeErr != nil || int64(len(data)) > limit || int64(len(data)) != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, history.Storage
	}
	return data, nil
}

func verificationDigest(path string, limit int64, private, missing bool) (string, bool, error) {
	file, before, err := verificationOpen(path, limit, private)
	if missing && os.IsNotExist(err) {
		sum := sha256.Sum256(nil)
		return hex.EncodeToString(sum[:]), false, nil
	}
	if err != nil {
		return "", false, history.Storage
	}
	hash := sha256.New()
	n, readErr := io.Copy(hash, io.LimitReader(file, limit+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil || statErr != nil || closeErr != nil || n > limit || n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return "", false, history.Storage
	}
	return hex.EncodeToString(hash.Sum(nil)), true, nil
}

func verificationSyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return history.Storage
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return history.Storage
	}
	return nil
}

// A run owns a fresh path. Never truncate/replace an existing continuation or
// follow its symlink. A failed partial file stays private, forbidding reuse.
func verificationCreate(path string, data []byte) error {
	if len(data) > verificationPlanLimit || !verificationDirectory(filepath.Dir(path)) {
		return history.Storage
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return history.Storage
	}
	n, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || n != len(data) || syncErr != nil || closeErr != nil {
		return history.Storage
	}
	return verificationSyncDirectory(filepath.Dir(path))
}

func verificationRemove(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > verificationPlanLimit {
		return history.Storage
	}
	if os.Remove(path) != nil {
		return history.Storage
	}
	return verificationSyncDirectory(filepath.Dir(path))
}
