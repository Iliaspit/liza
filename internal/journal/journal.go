// Package journal provides an append-only event journal for the blackboard.
//
// The journal records every state change as a typed event in a JSONL file
// next to state.yaml. During the migration period the journal is a shadow
// log: state.yaml remains the source of truth and events are derived from
// state diffs inside db.Blackboard write operations (which hold the state
// file lock, serializing appends).
//
// Event types reuse the task-history vocabulary from internal/models
// (e.g. "task.claimed", "task.rejected") plus structural events the history
// does not carry ("task.status_changed", "agent.registered", "anomaly.logged").
package journal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Filename is the journal file name, stored alongside state.yaml.
const Filename = "journal.jsonl"

// maxFieldBytes caps each string field value at the journal boundary so a
// misbehaving writer can never bloat the journal the way raw provider
// transcripts bloated state.yaml (the failure statehygiene exists for).
const maxFieldBytes = 4096

// truncationSuffix marks capped field values.
const truncationSuffix = "…[truncated by journal]"

// Event is a single journal entry.
type Event struct {
	Seq   int64     `json:"seq"`
	Time  time.Time `json:"time"`
	Type  string    `json:"type"`
	Task  string    `json:"task,omitempty"`
	Agent string    `json:"agent,omitempty"`
	// Op is the named write operation that produced this event (e.g.
	// "claim_task", "submit_verdict") — provenance for audit. Writes that
	// have not been migrated to named operations carry "modify" or "write".
	Op     string         `json:"op,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

// Store is an append-only JSONL event store.
//
// Concurrency: Append must be called while holding the state file lock
// (db.Blackboard guarantees this by appending inside its lock window).
// Reads scan the file without locking; a torn trailing line from a crashed
// writer is tolerated and skipped.
type Store struct {
	path string
}

// ForStatePath returns the Store for the journal living next to the given
// state.yaml path.
func ForStatePath(statePath string) *Store {
	return &Store{path: filepath.Join(filepath.Dir(statePath), Filename)}
}

// Path returns the journal file path.
func (s *Store) Path() string {
	return s.path
}

// Append assigns sequence numbers and timestamps, caps field sizes, and
// appends the events as JSON lines with fsync. The caller must hold the
// state file lock.
func (s *Store) Append(events []Event) error {
	if len(events) == 0 {
		return nil
	}

	last, err := s.LastSeq()
	if err != nil {
		return fmt.Errorf("journal: failed to determine last sequence: %w", err)
	}

	now := time.Now().UTC()
	var buf []byte
	for i := range events {
		events[i].Seq = last + int64(i) + 1
		if events[i].Time.IsZero() {
			events[i].Time = now
		}
		capFields(events[i].Fields)

		line, err := json.Marshal(events[i])
		if err != nil {
			return fmt.Errorf("journal: failed to marshal event %q: %w", events[i].Type, err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("journal: failed to open journal: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("journal: failed to append events: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("journal: failed to sync journal: %w", err)
	}
	return nil
}

// ReadAll returns every event in the journal in order. A torn trailing line
// (crash during append) is skipped; a malformed line anywhere else is an
// integrity error.
func (s *Store) ReadAll() ([]Event, error) {
	return s.ReadSince(0)
}

// ReadSince returns events with Seq strictly greater than seq, in order.
func (s *Store) ReadSince(seq int64) ([]Event, error) {
	return readEventsFile(s.path, seq)
}

// readEventsFile reads one JSONL event file, returning events with Seq
// strictly greater than seq. A missing file yields no events; a torn trailing
// line is tolerated; a malformed line anywhere else is an integrity error.
func readEventsFile(path string, seq int64) ([]Event, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("journal: failed to open journal: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var pendingErr error
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// A malformed line is only tolerable if it turns out to be the last
		// one (torn append); remember the error and fail if more lines follow.
		if pendingErr != nil {
			return nil, pendingErr
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			pendingErr = fmt.Errorf("journal: malformed event at %s line %d: %w", filepath.Base(path), lineNo, err)
			continue
		}
		if ev.Seq > seq {
			events = append(events, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("journal: failed to scan journal: %w", err)
	}
	return events, nil
}

// LastSeq returns the sequence number of the last well-formed event, or 0 for
// a missing or empty journal.
func (s *Store) LastSeq() (int64, error) {
	events, err := s.ReadAll()
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].Seq, nil
}

func capFields(fields map[string]any) {
	for k, v := range fields {
		str, ok := v.(string)
		if !ok || len(str) <= maxFieldBytes {
			continue
		}
		fields[k] = str[:maxFieldBytes] + truncationSuffix
	}
}
