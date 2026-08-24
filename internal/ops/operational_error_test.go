package ops

import (
	"errors"
	"fmt"
	"testing"
)

func TestOperationalError_Error(t *testing.T) {
	t.Parallel()

	t.Run("includes both Message and Err", func(t *testing.T) {
		inner := errors.New("permission denied")
		e := &OperationalError{Message: "failed to load config", Err: inner}
		got := e.Error()
		want := "failed to load config: permission denied"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("nil Err returns just Message", func(t *testing.T) {
		e := &OperationalError{Message: "failed to load config", Err: nil}
		got := e.Error()
		want := "failed to load config"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestOperationalError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("returns inner error", func(t *testing.T) {
		inner := errors.New("disk full")
		e := &OperationalError{Message: "write failed", Err: inner}
		if e.Unwrap() != inner {
			t.Error("Unwrap() did not return inner error")
		}
	})

	t.Run("nil Err returns nil", func(t *testing.T) {
		e := &OperationalError{Message: "write failed", Err: nil}
		if e.Unwrap() != nil {
			t.Error("Unwrap() should return nil when Err is nil")
		}
	})
}

func TestOperationalError_ErrorsAs_InnerType(t *testing.T) {
	t.Parallel()

	// OperationalError wrapping a PreconditionError — errors.As should
	// match the inner PreconditionError through the Unwrap chain.
	inner := &PreconditionError{Reason: "bad input"}
	outer := &OperationalError{Message: "op failed", Err: inner}
	wrapped := fmt.Errorf("handler: %w", outer)

	var pe *PreconditionError
	if !errors.As(wrapped, &pe) {
		t.Fatal("errors.As should find PreconditionError through OperationalError chain")
	}
	if pe.Reason != "bad input" {
		t.Errorf("Reason = %q, want %q", pe.Reason, "bad input")
	}

	var oe *OperationalError
	if !errors.As(wrapped, &oe) {
		t.Fatal("errors.As should also find OperationalError")
	}
	if oe.Message != "op failed" {
		t.Errorf("Message = %q, want %q", oe.Message, "op failed")
	}
}
