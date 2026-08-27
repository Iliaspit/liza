package ops

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAgentLifecycleLockSerializesOneAgent(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)

	firstAcquired := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithAgentLifecycleLock(context.Background(), projectRoot, "coder-1", "first", func() error {
			close(firstAcquired)
			<-releaseFirst
			return nil
		})
	}()
	<-firstAcquired

	sameAgentAcquired := make(chan struct{})
	sameAgentDone := make(chan error, 1)
	go func() {
		sameAgentDone <- WithAgentLifecycleLock(context.Background(), projectRoot, "coder-1", "second", func() error {
			close(sameAgentAcquired)
			return nil
		})
	}()

	otherAgentDone := make(chan error, 1)
	go func() {
		otherAgentDone <- WithAgentLifecycleLock(context.Background(), projectRoot, "coder-2", "other", func() error {
			return nil
		})
	}()
	select {
	case err := <-otherAgentDone:
		if err != nil {
			t.Fatalf("other agent lock: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("different agent lifecycle lock was unnecessarily serialized")
	}

	select {
	case <-sameAgentAcquired:
		t.Fatal("same agent lifecycle lock acquired before first holder released")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if err := <-sameAgentDone; err != nil {
		t.Fatalf("same agent lock after release: %v", err)
	}
}

func TestAgentLifecycleLockTimeoutReleasesWaiter(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	restore := SetAgentLifecycleLockTimeoutForTest(40 * time.Millisecond)
	t.Cleanup(restore)

	firstAcquired := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithAgentLifecycleLock(context.Background(), projectRoot, "coder-1", "holder", func() error {
			close(firstAcquired)
			<-releaseFirst
			return nil
		})
	}()
	<-firstAcquired

	err := WithAgentLifecycleLock(context.Background(), projectRoot, "coder-1", "waiter", func() error {
		t.Fatal("timed-out lifecycle callback ran")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "coder-1") || !strings.Contains(err.Error(), "within 40ms") {
		t.Fatalf("timeout error = %v, want bounded agent-specific diagnostic", err)
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("holder lock: %v", err)
	}
	if err := WithAgentLifecycleLock(context.Background(), projectRoot, "coder-1", "after-timeout", func() error { return nil }); err != nil {
		t.Fatalf("lock remained held after timeout: %v", err)
	}
}
