package db

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/filelock"
	"github.com/liza-mas/liza/internal/journal"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/statehygiene"
	"github.com/liza-mas/liza/internal/taskinvariants"
	"gopkg.in/yaml.v3"
)

// instances holds per-path singleton Blackboard instances.
// All production code should use For() to get a shared instance.
var instances sync.Map

var unsafeBlockScalarIndentRE = regexp.MustCompile(`^(.*?)([|>])([1-9])([+-]?)$`)

// Blackboard provides thread-safe access to the state.yaml file
type Blackboard struct {
	statePath string
	fileLock  *filelock.FileLock

	// Cache fields for performance optimization
	// We cache raw YAML bytes (not a parsed struct) so that each ReadCached
	// call returns a fresh *models.State. This prevents callers from silently
	// corrupting a shared cached struct.
	cacheMu     sync.RWMutex
	cachedData  []byte
	cachedMtime time.Time

	// Structural-invariant classifier for write-funnel enforcement, loaded
	// lazily from the frozen pipeline config. nil when the config is absent
	// (legacy projects, unit tests) — enforcement is then skipped.
	invariantsOnce sync.Once
	invariants     *taskinvariants.StatusClassifier
}

// New creates a Blackboard backed by the given state file path.
// Use For() in production code to get a shared process-level singleton.
// New is intended for tests that need independent instances.
func New(statePath string) *Blackboard {
	return &Blackboard{
		statePath: statePath,
		fileLock:  filelock.New(statePath),
	}
}

// For returns a process-level singleton Blackboard for the given state path.
// All callers sharing the same path within a process get the same instance,
// ensuring cache coherence and preventing state fragmentation if Blackboard
// gains in-process state in the future.
//
// The statePath is cleaned via filepath.Clean to ensure callers using
// equivalent paths (e.g. with trailing slashes) share the same instance.
func For(statePath string) *Blackboard {
	key := filepath.Clean(statePath)
	if v, ok := instances.Load(key); ok {
		return v.(*Blackboard)
	}
	bb := New(key)
	actual, _ := instances.LoadOrStore(key, bb)
	return actual.(*Blackboard)
}

// ResetInstances clears all cached singleton instances.
// Intended for test cleanup only.
func ResetInstances() {
	instances.Range(func(key, _ any) bool {
		instances.Delete(key)
		return true
	})
}

// WithLockTimeout creates a new independent instance with a custom lock timeout;
// cached bytes are copied at creation time but diverge afterward. The returned
// instance is intentionally NOT registered in the singleton map — it is a
// short-lived specialization for callers that need different lock behavior.
func (bb *Blackboard) WithLockTimeout(timeout time.Duration) *Blackboard {
	bb.cacheMu.RLock()
	cachedData := bb.cachedData
	cachedMtime := bb.cachedMtime
	bb.cacheMu.RUnlock()

	newBB := &Blackboard{
		statePath:   bb.statePath,
		fileLock:    filelock.New(bb.statePath).WithTimeout(timeout),
		cachedData:  cachedData,
		cachedMtime: cachedMtime,
	}
	return newBB
}

// EnableMetrics enables lock metrics collection.
func (bb *Blackboard) EnableMetrics() {
	bb.fileLock.EnableMetrics()
}

// DisableMetrics disables lock metrics collection.
func (bb *Blackboard) DisableMetrics() {
	bb.fileLock.DisableMetrics()
}

// GetMetricsRecorder returns the metrics recorder, or nil if not enabled.
func (bb *Blackboard) GetMetricsRecorder() *filelock.MetricsRecorder {
	return bb.fileLock.GetMetricsRecorder()
}

// Read returns the current state under an exclusive file lock.
func (bb *Blackboard) Read() (*models.State, error) {
	var state models.State
	err := bb.fileLock.WithLockOperation("read", func() error {
		data, err := os.ReadFile(bb.statePath)
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal(data, &state); err != nil {
			return &errors.StateSchemaError{Operation: "state read", Err: err}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	normalizeAgentRoles(&state)
	normalizeTaskAttempts(&state)
	return &state, nil
}

// ReadRaw reads the raw state.yaml bytes under flock protection.
// Use this when you need the file content without parsing (e.g., serving
// the raw YAML to an external consumer), while still respecting the lock
// to avoid reading partially-written data.
func (bb *Blackboard) ReadRaw() ([]byte, error) {
	var data []byte
	err := bb.fileLock.WithLockOperation("read-raw", func() error {
		var readErr error
		data, readErr = os.ReadFile(bb.statePath)
		return readErr
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadCached reads the current state with caching based on file mtime.
// This method avoids disk I/O when the file hasn't changed by caching raw
// YAML bytes. Each call returns a freshly-parsed *models.State, so callers
// can safely mutate the result without corrupting other readers.
func (bb *Blackboard) ReadCached() (*models.State, error) {
	fileInfo, err := os.Stat(bb.statePath)
	if err != nil {
		bb.InvalidateCache()
		return nil, err
	}

	currentMtime := fileInfo.ModTime()

	bb.cacheMu.RLock()
	cachedData := bb.cachedData
	cachedMtime := bb.cachedMtime
	bb.cacheMu.RUnlock()

	var data []byte
	if cachedData != nil && currentMtime.Equal(cachedMtime) {
		data = cachedData
	} else {
		data, err = os.ReadFile(bb.statePath)
		if err != nil {
			bb.InvalidateCache()
			return nil, err
		}

		bb.cacheMu.Lock()
		bb.cachedData = data
		bb.cachedMtime = currentMtime
		bb.cacheMu.Unlock()
	}

	var state models.State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, &errors.StateSchemaError{Operation: "state read cached", Err: err}
	}

	normalizeAgentRoles(&state)
	normalizeTaskAttempts(&state)
	return &state, nil
}

// InvalidateCache forces the next ReadCached call to reload from disk.
func (bb *Blackboard) InvalidateCache() {
	bb.cacheMu.Lock()
	bb.cachedData = nil
	bb.cachedMtime = time.Time{}
	bb.cacheMu.Unlock()
}

// writeStateData writes data to the state file atomically using fsync + rename.
// Must be called while holding the file lock.
// Uses a unique temp file per call to avoid races if the file lock has gaps.
func (bb *Blackboard) writeStateData(data []byte) error {
	dir := filepath.Dir(bb.statePath)
	base := filepath.Base(bb.statePath)

	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}
	tmpPath := f.Name()

	// CreateTemp uses 0600; match the target file permissions
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set temporary file permissions: %w", err)
	}

	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()

	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write state data: %w", writeErr)
	}
	if syncErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync state file: %w", syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close state file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, bb.statePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

func yamlBytesParse(data []byte) error {
	var probe any
	return yaml.Unmarshal(data, &probe)
}

func rewriteUnsafeBlockScalarIndents(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+4)
	for i, line := range lines {
		m := unsafeBlockScalarIndentRE.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}

		explicitIndent, err := strconv.Atoi(m[3])
		if err != nil {
			out = append(out, line)
			continue
		}

		nextIndent, found := nextNonEmptyLineIndent(lines, i+1)
		if !found {
			out = append(out, line)
			continue
		}

		requiredIndent := len(leadingWhitespace(line)) + explicitIndent
		if len(nextIndent) >= requiredIndent {
			out = append(out, line)
			continue
		}

		out = append(out, m[1]+m[2]+m[4])
		out = append(out, nextIndent)
	}
	return []byte(strings.Join(out, "\n"))
}

func nextNonEmptyLineIndent(lines []string, start int) (string, bool) {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return leadingWhitespace(lines[i]), true
	}
	return "", false
}

func leadingWhitespace(s string) string {
	idx := len(s) - len(strings.TrimLeft(s, " \t"))
	if idx <= 0 {
		return ""
	}
	return s[:idx]
}

func marshalStateForWrite(state *models.State) ([]byte, error) {
	if err := statehygiene.ValidateState(state); err != nil {
		return nil, fmt.Errorf("state hygiene validation failed: %w", err)
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}
	if err := yamlBytesParse(data); err == nil {
		return data, nil
	}

	// Why: go-yaml can emit broken explicit indent indicators such as `|4-`
	// for leading-blank-line scalars; rewriting only the header preserves the
	// original text while keeping state.yaml parseable.
	rewritten := rewriteUnsafeBlockScalarIndents(data)
	if err := yamlBytesParse(rewritten); err != nil {
		return nil, fmt.Errorf("failed to marshal parseable state YAML: %w", err)
	}
	return rewritten, nil
}

// Write writes the state to the state file atomically with fsync
func (bb *Blackboard) Write(state *models.State) error {
	err := bb.fileLock.WithLockOperation("write", func() error {
		before, beforeOK := bb.readStateForDiff()

		data, err := marshalStateForWrite(state)
		if err != nil {
			return err
		}
		if err := bb.writeStateData(data); err != nil {
			return err
		}

		if beforeOK {
			bb.appendDerivedEvents("write", before, state)
		}
		return nil
	})

	if err == nil {
		bb.InvalidateCache()
	}

	return err
}

// WriteInvariantError reports a named operation that attempted to write a
// task violating a structural invariant. The write is aborted — state.yaml
// is untouched.
type WriteInvariantError struct {
	Op     string
	TaskID string
	Err    error
}

func (e *WriteInvariantError) Error() string {
	return fmt.Sprintf("operation %q rejected: would write structurally invalid task %s: %v", e.Op, e.TaskID, e.Err)
}

func (e *WriteInvariantError) Unwrap() error { return e.Err }

// checkWriteInvariants enforces structural per-task invariants at the write
// funnel for named operations with no-worse-than-before semantics: an
// operation may not INTRODUCE a structural violation (create an invalid task,
// or turn a valid task invalid), but touching a task that was already invalid
// is allowed — otherwise pre-existing corruption would wedge the very
// operations that repair it.
//
// Fail-open cases, deliberate during the strangler migration:
//   - generic "modify"/"write" operations (unmigrated call sites, fixtures);
//   - no frozen pipeline config (legacy projects, most unit tests).
func (bb *Blackboard) checkWriteInvariants(op string, before, after *models.State) error {
	if op == "modify" || op == "write" {
		return nil
	}
	sc := bb.statusClassifier()
	if sc == nil {
		return nil
	}

	beforeTasks := make(map[string]*models.Task, len(before.Tasks))
	for i := range before.Tasks {
		beforeTasks[before.Tasks[i].ID] = &before.Tasks[i]
	}
	for i := range after.Tasks {
		task := &after.Tasks[i]
		prev := beforeTasks[task.ID]
		if prev != nil && reflect.DeepEqual(*prev, *task) {
			continue
		}
		err := taskinvariants.ValidateStatusFields(task, sc)
		if err == nil {
			continue
		}
		if prev != nil && taskinvariants.ValidateStatusFields(prev, sc) != nil {
			// Pre-existing violation on this task; the op did not introduce it.
			continue
		}
		return &WriteInvariantError{Op: op, TaskID: task.ID, Err: err}
	}
	return nil
}

// statusClassifier lazily loads the structural classifier from the frozen
// pipeline config next to the state file. Returns nil when unavailable.
func (bb *Blackboard) statusClassifier() *taskinvariants.StatusClassifier {
	bb.invariantsOnce.Do(func() {
		projectRoot := filepath.Dir(filepath.Dir(bb.statePath))
		cfg, err := pipeline.LoadFrozen(projectRoot)
		if err != nil {
			return
		}
		sc := taskinvariants.NewStatusClassifier(pipeline.NewResolver(cfg), cfg)
		bb.invariants = &sc
	})
	return bb.invariants
}

// readStateForDiff reads and parses the current state file as the "before"
// snapshot for journal derivation. A missing file is a valid empty before
// state (project initialization); any other failure disables journaling for
// this write rather than failing the state operation.
func (bb *Blackboard) readStateForDiff() (*models.State, bool) {
	data, err := os.ReadFile(bb.statePath)
	if os.IsNotExist(err) {
		return &models.State{}, true
	}
	if err != nil {
		return nil, false
	}
	var state models.State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, false
	}
	normalizeAgentRoles(&state)
	normalizeTaskAttempts(&state)
	return &state, true
}

// appendDerivedEvents derives journal events from a state transition and
// appends them to the shadow journal. Must be called while holding the state
// file lock so appends serialize with state writes.
//
// During the shadow-journal phase state.yaml remains the source of truth, so
// an append failure must not fail (or invite a retry of) a state write that
// already succeeded — it is reported on stderr and surfaces in
// `liza journal --verify` instead.
func (bb *Blackboard) appendDerivedEvents(op string, before, after *models.State) {
	events := journal.Derive(before, after)
	if len(events) == 0 {
		return
	}
	for i := range events {
		events[i].Op = op
	}
	store := journal.ForStatePath(bb.statePath)
	if err := store.Append(events); err != nil {
		fmt.Fprintf(os.Stderr, "warning: journal append failed (state write succeeded): %v\n", err)
		return
	}
	// Rotation check is O(1) per append: MaybeRotate stats the file and only
	// counts events once the size heuristic says the threshold may be
	// exceeded. We hold the state file lock here, satisfying Rotate's
	// contract. Failures are non-fatal — the journal just keeps growing and
	// the next append retries.
	if _, err := store.MaybeRotate(journal.DefaultRotateThreshold); err != nil {
		fmt.Fprintf(os.Stderr, "warning: journal rotation failed (state write succeeded): %v\n", err)
	}
}

// Modify performs an atomic read-modify-write operation under the generic
// "modify" operation name. Prefer ModifyOp at call sites that represent a
// named lifecycle operation — the name flows into lock-owner diagnostics and
// journal event provenance.
func (bb *Blackboard) Modify(fn func(*models.State) error) error {
	return bb.ModifyOp("modify", fn)
}

// ModifyOp performs an atomic read-modify-write operation attributed to the
// named operation.
func (bb *Blackboard) ModifyOp(op string, fn func(*models.State) error) error {
	if op == "" {
		op = "modify"
	}
	err := bb.fileLock.WithLockOperation(op, func() error {
		data, err := os.ReadFile(bb.statePath)
		if err != nil {
			return fmt.Errorf("failed to read state: %w", err)
		}

		var state models.State
		if err := yaml.Unmarshal(data, &state); err != nil {
			return &errors.StateSchemaError{Operation: "state modify", Err: err}
		}

		// Second unmarshal of the same bytes: an independent "before" snapshot
		// for journal derivation that fn cannot reach through shared pointers.
		var before models.State
		if err := yaml.Unmarshal(data, &before); err != nil {
			return &errors.StateSchemaError{Operation: "state modify", Err: err}
		}

		normalizeAgentRoles(&state)
		normalizeTaskAttempts(&state)
		normalizeAgentRoles(&before)
		normalizeTaskAttempts(&before)

		if err := fn(&state); err != nil {
			return fmt.Errorf("modification function failed: %w", err)
		}

		if err := bb.checkWriteInvariants(op, &before, &state); err != nil {
			return err
		}

		data, err = marshalStateForWrite(&state)
		if err != nil {
			return err
		}
		if err := bb.writeStateData(data); err != nil {
			return err
		}

		bb.appendDerivedEvents(op, &before, &state)
		return nil
	})

	if err == nil {
		bb.InvalidateCache()
	}

	return err
}

// GetTask returns the task with the given ID, or (nil, nil) if not found.
func (bb *Blackboard) GetTask(taskID string) (*models.Task, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, err
	}

	return state.FindTask(taskID), nil
}

// GetAgent returns the agent with the given ID, or (nil, nil) if not found.
func (bb *Blackboard) GetAgent(agentID string) (*models.Agent, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, err
	}

	if agent, ok := state.Agents[agentID]; ok {
		return &agent, nil
	}

	return nil, nil
}

// UpdateTask atomically updates a task by ID
func (bb *Blackboard) UpdateTask(taskID string, fn func(*models.Task) error) error {
	return bb.ModifyOp("update_task", func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		return fn(task)
	})
}

// UpdateAgent atomically updates an agent by ID
func (bb *Blackboard) UpdateAgent(agentID string, fn func(*models.Agent) error) error {
	return bb.ModifyOp("update_agent", func(state *models.State) error {
		agent, ok := state.Agents[agentID]
		if !ok {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		if err := fn(&agent); err != nil {
			return err
		}

		state.Agents[agentID] = agent
		return nil
	})
}

// GetStatePath returns the path to the state file.
func (bb *Blackboard) GetStatePath() string {
	return bb.statePath
}

// normalizeTaskAttempts converts the legacy attempted: list into the Attempt
// field in-memory. Does not write back to disk — normalization is read-path only.
func normalizeTaskAttempts(state *models.State) {
	for i := range state.Tasks {
		state.Tasks[i].MigrateAttemptedField()
	}
}

// normalizeAgentRoles converts legacy underscore-form role names to hyphenated
// form in-memory. Does not write back to disk — normalization is read-path only.
func normalizeAgentRoles(state *models.State) {
	for id, agent := range state.Agents {
		normalized := roles.NormalizeRoleName(agent.Role)
		if normalized != agent.Role {
			agent.Role = normalized
			state.Agents[id] = agent
		}
	}
}
