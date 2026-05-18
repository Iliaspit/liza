package filelock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

const (
	// DefaultLockTimeout is the default maximum time to wait for a file lock.
	DefaultLockTimeout = 10 * time.Second
	// LockCheckInterval is how often to retry lock acquisition.
	LockCheckInterval = 100 * time.Millisecond
)

// FileLock provides file-based mutual exclusion with diagnostic owner metadata.
//
// It wraps flock(2) with a polling acquisition loop, best-effort owner metadata,
// classified error types, and optional metrics collection.
type FileLock struct {
	lockPath    string
	ownerPath   string
	lockTimeout time.Duration

	// Metrics collection (optional)
	metricsRecorder *MetricsRecorder
	enableMetrics   bool
}

// New creates a FileLock that protects the given file path.
// Lock file: protectedPath + ".lock", owner metadata: protectedPath + ".lock.owner.json".
func New(protectedPath string) *FileLock {
	return &FileLock{
		lockPath:    protectedPath + ".lock",
		ownerPath:   protectedPath + ".lock.owner.json",
		lockTimeout: DefaultLockTimeout,
	}
}

// WithTimeout returns a new FileLock with the given timeout.
// Metrics state is not shared with the original.
func (fl *FileLock) WithTimeout(timeout time.Duration) *FileLock {
	return &FileLock{
		lockPath:    fl.lockPath,
		ownerPath:   fl.ownerPath,
		lockTimeout: timeout,
	}
}

// EnableMetrics enables lock metrics collection.
func (fl *FileLock) EnableMetrics() {
	if fl.metricsRecorder == nil {
		fl.metricsRecorder = NewMetricsRecorder()
	}
	fl.enableMetrics = true
}

// DisableMetrics disables lock metrics collection.
func (fl *FileLock) DisableMetrics() {
	fl.enableMetrics = false
}

// GetMetricsRecorder returns the metrics recorder, or nil if not enabled.
func (fl *FileLock) GetMetricsRecorder() *MetricsRecorder {
	return fl.metricsRecorder
}

type ownerMetadata struct {
	Version    int    `json:"version"`
	PID        int    `json:"pid"`
	Hostname   string `json:"hostname,omitempty"`
	Operation  string `json:"operation"`
	AcquiredAt string `json:"acquired_at"`
}

func (fl *FileLock) acquireLock(operation string) (*flock.Flock, error) {
	lock := flock.New(fl.lockPath)
	acquired, err := lock.TryLock()
	if err != nil {
		return nil, ClassifyLockError(err)
	}
	if !acquired {
		return nil, fmt.Errorf("lock not acquired")
	}

	fl.writeOwnerMetadata(operation)

	return lock, nil
}

func (fl *FileLock) writeOwnerMetadata(operation string) {
	hostname, _ := os.Hostname()
	metadata := ownerMetadata{
		Version:    1,
		PID:        os.Getpid(),
		Hostname:   hostname,
		Operation:  operation,
		AcquiredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return
	}
	data = append(data, '\n')

	dir := filepath.Dir(fl.ownerPath)
	base := filepath.Base(fl.ownerPath)
	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpPath, fl.ownerPath)
}

// WithLock executes fn while holding an exclusive file lock.
// Equivalent to WithLockOperation with operation "unknown".
func (fl *FileLock) WithLock(fn func() error) error {
	return fl.WithLockOperation("unknown", fn)
}

// WithLockOperation executes fn while holding an exclusive file lock.
// The operation name is recorded in metrics if enabled.
func (fl *FileLock) WithLockOperation(operation string, fn func() error) error {
	var lock *flock.Flock
	var err error

	acquireStart := time.Now()
	deadline := acquireStart.Add(fl.lockTimeout)
	locked := false

	for time.Now().Before(deadline) {
		lock, err = fl.acquireLock(operation)
		if err == nil {
			locked = true
			break
		}
		// If it's a non-retryable error (permission, disk full, etc.), fail immediately
		var lockErr *LockError
		if errors.As(err, &lockErr) {
			switch lockErr.Type {
			case LockErrorPermission, LockErrorDiskFull, LockErrorFilesystem:
				return lockErr
			}
		}
		time.Sleep(LockCheckInterval)
	}

	if !locked {
		return NewLockTimeout(fmt.Errorf("lock unavailable after %v", fl.lockTimeout))
	}

	acquisitionTime := time.Since(acquireStart)
	holdStart := time.Now()

	// We intentionally do NOT remove the lock file or owner metadata here.
	// Removing the lock file after unlock creates a race: another process can
	// create a new file (different inode) and acquire flock on it, then this
	// process deletes that file, allowing a third process to create yet another
	// file — resulting in two processes holding flock on different inodes
	// simultaneously. Leaving the file in place ensures all processes flock
	// the same inode.
	defer func() {
		lock.Unlock()

		if fl.enableMetrics && fl.metricsRecorder != nil {
			holdTime := time.Since(holdStart)
			fl.metricsRecorder.Record(&Metrics{
				Operation:       operation,
				AcquisitionTime: acquisitionTime,
				HoldTime:        holdTime,
			})
		}
	}()

	return fn()
}
