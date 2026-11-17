//go:build !darwin && !linux

package localhistory

import (
	"os"

	"my-bilibili/internal/history"
)

const lockSupported = false

func openNoFollow(string, int, os.FileMode) (*os.File, error) {
	return nil, history.LockUnsupported
}
func acquireLock(*os.File) error { return history.LockUnsupported }
func releaseLock(*os.File) error { return history.LockUnsupported }
