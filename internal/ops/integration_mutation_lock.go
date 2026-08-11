package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/filelock"
)

// The protected CAS and index-sync window is normally sub-second. Thirty
// seconds absorbs a queue of concurrent merges while bounding cold-cache or
// unusually large checkouts; a timeout remains a retryable merge error.
const integrationMutationLockTimeout = 30 * time.Second

func withIntegrationMutationLock(projectRoot, operation string, fn func() error) error {
	return withIntegrationMutationLockTimeout(projectRoot, operation, integrationMutationLockTimeout, fn)
}

func withIntegrationMutationLockTimeout(projectRoot, operation string, timeout time.Duration, fn func() error) error {
	lock, err := projectFileLock(projectRoot, "integration-mutation")
	if err != nil {
		return err
	}

	// Lock ordering is integration mutation lock -> blackboard read lock.
	// Callers must release this lock before any blackboard state write.
	err = lock.WithTimeout(timeout).WithLockOperation(operation, fn)
	if filelock.IsLockErrorType(err, filelock.LockErrorTimeout) {
		return fmt.Errorf("integration mutation lock operation %q could not acquire the lock within %s; another merge is updating the integration ref or main index; retry the merge: %w", operation, timeout, err)
	}
	return err
}
