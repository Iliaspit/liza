package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestLockErrorType(t *testing.T) {
	tests := []struct {
		name     string
		lockErr  *LockError
		wantType LockErrorType
	}{
		{
			name: "timeout error",
			lockErr: &LockError{
				Type:    LockErrorTimeout,
				Message: "lock acquisition timed out",
				Err:     errors.New("timeout"),
			},
			wantType: LockErrorTimeout,
		},
		{
			name: "permission error",
			lockErr: &LockError{
				Type:    LockErrorPermission,
				Message: "permission denied",
				Err:     syscall.EACCES,
			},
			wantType: LockErrorPermission,
		},
		{
			name: "disk full error",
			lockErr: &LockError{
				Type:    LockErrorDiskFull,
				Message: "no space left on device",
				Err:     syscall.ENOSPC,
			},
			wantType: LockErrorDiskFull,
		},
		{
			name: "filesystem error",
			lockErr: &LockError{
				Type:    LockErrorFilesystem,
				Message: "I/O error",
				Err:     syscall.EIO,
			},
			wantType: LockErrorFilesystem,
		},
		{
			name: "stale owner metadata error",
			lockErr: &LockError{
				Type:    LockErrorStale,
				Message: "owner metadata references dead process",
				Err:     errors.New("stale owner metadata"),
			},
			wantType: LockErrorStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lockErr.Type != tt.wantType {
				t.Errorf("LockError.Type = %v, want %v", tt.lockErr.Type, tt.wantType)
			}
		})
	}
}

func TestLockErrorError(t *testing.T) {
	tests := []struct {
		name    string
		lockErr *LockError
		want    string
	}{
		{
			name: "with underlying error",
			lockErr: &LockError{
				Type:    LockErrorTimeout,
				Message: "lock acquisition timed out",
				Err:     errors.New("timeout after 10s"),
			},
			want: "lock error (timeout): lock acquisition timed out: timeout after 10s",
		},
		{
			name: "without underlying error",
			lockErr: &LockError{
				Type:    LockErrorPermission,
				Message: "permission denied",
				Err:     nil,
			},
			want: "lock error (permission): permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.lockErr.Error()
			if got != tt.want {
				t.Errorf("LockError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLockErrorUnwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	lockErr := &LockError{
		Type:    LockErrorTimeout,
		Message: "timeout",
		Err:     underlyingErr,
	}

	unwrapped := errors.Unwrap(lockErr)
	if unwrapped != underlyingErr {
		t.Errorf("errors.Unwrap() returned %v, want %v", unwrapped, underlyingErr)
	}
}

func TestLockErrorIs(t *testing.T) {
	underlyingErr := syscall.EACCES
	lockErr := &LockError{
		Type:    LockErrorPermission,
		Message: "permission denied",
		Err:     underlyingErr,
	}

	if !errors.Is(lockErr, syscall.EACCES) {
		t.Error("errors.Is() should match underlying syscall.EACCES")
	}
}

func TestIsLockErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType LockErrorType
		want     bool
	}{
		{
			name: "matching timeout error",
			err: &LockError{
				Type:    LockErrorTimeout,
				Message: "timeout",
			},
			wantType: LockErrorTimeout,
			want:     true,
		},
		{
			name: "non-matching error type",
			err: &LockError{
				Type:    LockErrorPermission,
				Message: "permission denied",
			},
			wantType: LockErrorTimeout,
			want:     false,
		},
		{
			name:     "non-LockError",
			err:      errors.New("generic error"),
			wantType: LockErrorTimeout,
			want:     false,
		},
		{
			name:     "nil error",
			err:      nil,
			wantType: LockErrorTimeout,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLockErrorType(tt.err, tt.wantType)
			if got != tt.want {
				t.Errorf("IsLockErrorType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyLockError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType LockErrorType
		wantMsg  string
	}{
		{
			name:     "permission denied EACCES",
			err:      syscall.EACCES,
			wantType: LockErrorPermission,
			wantMsg:  "permission denied",
		},
		{
			name:     "permission denied EPERM",
			err:      syscall.EPERM,
			wantType: LockErrorPermission,
			wantMsg:  "operation not permitted",
		},
		{
			name:     "disk full ENOSPC",
			err:      syscall.ENOSPC,
			wantType: LockErrorDiskFull,
			wantMsg:  "no space left on device",
		},
		{
			name:     "I/O error EIO",
			err:      syscall.EIO,
			wantType: LockErrorFilesystem,
			wantMsg:  "I/O error",
		},
		{
			name:     "read-only filesystem EROFS",
			err:      syscall.EROFS,
			wantType: LockErrorFilesystem,
			wantMsg:  "read-only file system",
		},
		{
			name:     "path error with EACCES",
			err:      &os.PathError{Op: "open", Path: "/test", Err: syscall.EACCES},
			wantType: LockErrorPermission,
			wantMsg:  "permission denied",
		},
		{
			name:     "link error with ENOSPC",
			err:      &os.LinkError{Op: "symlink", Old: "/a", New: "/b", Err: syscall.ENOSPC},
			wantType: LockErrorDiskFull,
			wantMsg:  "no space left on device",
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			wantType: LockErrorFilesystem,
			wantMsg:  "filesystem error",
		},
		{
			name:     "nil error",
			err:      nil,
			wantType: LockErrorFilesystem,
			wantMsg:  "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockErr := ClassifyLockError(tt.err)

			if lockErr.Type != tt.wantType {
				t.Errorf("ClassifyLockError().Type = %v, want %v", lockErr.Type, tt.wantType)
			}

			if lockErr.Message != tt.wantMsg {
				t.Errorf("ClassifyLockError().Message = %q, want %q", lockErr.Message, tt.wantMsg)
			}

			if lockErr.Err != tt.err {
				t.Errorf("ClassifyLockError().Err = %v, want %v", lockErr.Err, tt.err)
			}
		})
	}
}

func TestNewLockTimeout(t *testing.T) {
	err := errors.New("timeout waiting for lock")
	lockErr := NewLockTimeout(err)

	if lockErr.Type != LockErrorTimeout {
		t.Errorf("NewLockTimeout().Type = %v, want %v", lockErr.Type, LockErrorTimeout)
	}

	if lockErr.Err != err {
		t.Errorf("NewLockTimeout().Err = %v, want %v", lockErr.Err, err)
	}

	if lockErr.Message != "lock acquisition timed out" {
		t.Errorf("NewLockTimeout().Message = %q, want %q", lockErr.Message, "lock acquisition timed out")
	}
}

func TestNewLockStale(t *testing.T) {
	pid := 12345
	lockErr := NewLockStale(pid)

	if lockErr.Type != LockErrorStale {
		t.Errorf("NewLockStale().Type = %v, want %v", lockErr.Type, LockErrorStale)
	}

	expectedMsg := fmt.Sprintf("owner metadata references dead process (PID %d)", pid)
	if lockErr.Message != expectedMsg {
		t.Errorf("NewLockStale().Message = %q, want %q", lockErr.Message, expectedMsg)
	}
}

func TestLockErrorTypeString(t *testing.T) {
	tests := []struct {
		errType LockErrorType
		want    string
	}{
		{LockErrorTimeout, "timeout"},
		{LockErrorPermission, "permission"},
		{LockErrorDiskFull, "disk_full"},
		{LockErrorFilesystem, "filesystem"},
		{LockErrorStale, "stale"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.errType.String()
			if got != tt.want {
				t.Errorf("LockErrorType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
