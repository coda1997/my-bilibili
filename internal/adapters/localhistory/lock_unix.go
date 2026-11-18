//go:build darwin || linux

package localhistory

import (
	"errors"
	"os"
	"syscall"

	"my-bilibili/internal/history"
)

const lockSupported = true

func openNoFollow(path string, flags int, mode os.FileMode) (*os.File, error) {
	// Nonblocking also prevents an unexpected FIFO from hanging before Stat.
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, mode)
}

func acquireLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return history.InUse
	}
	return err
}

func releaseLock(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
