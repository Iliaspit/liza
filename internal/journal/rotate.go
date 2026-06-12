package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EventJournalRotated is the snapshot event that seeds a fresh journal after
// rotation. Its Fields carry:
//
//	archived_file        — archive path relative to the journal directory
//	archived_through_seq — Seq of the last event moved to the archive
//	task_statuses        — full ProjectTaskStatuses fold of everything archived
//
// Projections seed from task_statuses so folding the live journal alone stays
// exactly equivalent to folding the full (archives + live) history.
const EventJournalRotated = "journal.rotated"

// ArchiveDirName is the directory (next to the journal) holding rotated
// journal segments.
const ArchiveDirName = "journal-archive"

// DefaultRotateThreshold is the event count above which the automatic
// rotation trigger in db.Blackboard rotates the journal.
const DefaultRotateThreshold int64 = 10000

// minEventBytes is a deliberately LOW estimate of the smallest realistic
// journal line (a minimal event — seq, RFC3339 time, type — is ~80 bytes;
// typical derived events run 150–300). MaybeRotate uses it as a cheap size
// gate: a journal smaller than threshold*minEventBytes cannot plausibly hold
// more than threshold events, so the per-append cost stays at one os.Stat.
// Because the estimate is low, by the time the gate opens the real count is
// almost certainly over the threshold, so full-file counting scans are rare.
const minEventBytes = 64

// RotateResult describes a completed rotation.
type RotateResult struct {
	// ArchivedFile is the absolute path of the archive segment.
	ArchivedFile string
	// ArchivedThroughSeq is the Seq of the last archived event; the snapshot
	// event seeding the fresh journal carries ArchivedThroughSeq+1.
	ArchivedThroughSeq int64
	// ArchivedEvents is the number of events moved to the archive.
	ArchivedEvents int
}

// Rotate archives the current journal when it holds more than threshold
// events and starts a fresh one seeded with a single EventJournalRotated
// snapshot event. Returns (nil, nil) when the journal is at or under the
// threshold.
//
// The archived file moves to journal-archive/journal-<firstseq>-<lastseq>.jsonl
// (zero-padded so lexicographic order matches sequence order). The snapshot
// event continues the sequence at lastseq+1 — LastSeq keeps increasing
// monotonically across rotation, never resetting.
//
// Concurrency: the caller must hold the state file lock — the same contract
// as Append (db.Blackboard rotates inside its lock window).
//
// Crash safety: the fresh journal is fully written and fsynced to a temp file
// before the current journal is renamed into the archive, so the only crash
// window is between the two renames; ReadAllIncludingArchives still sees the
// full history in that case.
func (s *Store) Rotate(threshold int64) (*RotateResult, error) {
	events, err := s.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("journal: rotate failed to read journal: %w", err)
	}
	if int64(len(events)) <= threshold {
		return nil, nil
	}

	firstSeq := events[0].Seq
	lastSeq := events[len(events)-1].Seq

	dir := filepath.Dir(s.path)
	archiveDir := filepath.Join(dir, ArchiveDirName)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("journal: failed to create archive directory: %w", err)
	}
	archiveName := fmt.Sprintf("journal-%012d-%012d.jsonl", firstSeq, lastSeq)
	archivePath := filepath.Join(archiveDir, archiveName)

	snapshot := Event{
		Seq:  lastSeq + 1,
		Time: time.Now().UTC(),
		Type: EventJournalRotated,
		Fields: map[string]any{
			"archived_file":        filepath.Join(ArchiveDirName, archiveName),
			"archived_through_seq": lastSeq,
			"task_statuses":        map[string]string(ProjectTaskStatuses(events)),
		},
	}
	line, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("journal: failed to marshal rotation snapshot: %w", err)
	}

	tmpPath, err := writeFreshJournal(dir, append(line, '\n'))
	if err != nil {
		return nil, err
	}

	if err := os.Rename(s.path, archivePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("journal: failed to archive journal: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("journal: failed to install fresh journal: %w", err)
	}

	return &RotateResult{
		ArchivedFile:       archivePath,
		ArchivedThroughSeq: lastSeq,
		ArchivedEvents:     len(events),
	}, nil
}

// writeFreshJournal writes and fsyncs the seeded journal content to a temp
// file in dir, returning its path.
func writeFreshJournal(dir string, content []byte) (string, error) {
	f, err := os.CreateTemp(dir, Filename+".rotate.*")
	if err != nil {
		return "", fmt.Errorf("journal: failed to create fresh journal: %w", err)
	}
	tmpPath := f.Name()
	cleanup := func(err error) (string, error) {
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := f.Chmod(0644); err != nil {
		return cleanup(fmt.Errorf("journal: failed to set fresh journal permissions: %w", err))
	}
	if _, err := f.Write(content); err != nil {
		return cleanup(fmt.Errorf("journal: failed to write fresh journal: %w", err))
	}
	if err := f.Sync(); err != nil {
		return cleanup(fmt.Errorf("journal: failed to sync fresh journal: %w", err))
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("journal: failed to close fresh journal: %w", err)
	}
	return tmpPath, nil
}

// MaybeRotate rotates only when the journal plausibly exceeds threshold
// events, at O(1) cost per call: it stats the file and skips entirely while
// the size is under threshold*minEventBytes. Only past that gate does it pay
// for the full event count (inside Rotate, which no-ops if the count is still
// at or under the threshold).
//
// Same lock contract as Rotate: the caller must hold the state file lock.
func (s *Store) MaybeRotate(threshold int64) (*RotateResult, error) {
	fi, err := os.Stat(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("journal: failed to stat journal: %w", err)
	}
	if fi.Size() < threshold*minEventBytes {
		return nil, nil
	}
	return s.Rotate(threshold)
}

// ReadAllIncludingArchives returns the full event history for audit: every
// archived segment in sequence order, then the live journal. The snapshot
// events written by Rotate appear in-stream; their task_statuses seed equals
// the fold of everything before them, so projections over this stream match
// projections over the live journal alone.
func (s *Store) ReadAllIncludingArchives() ([]Event, error) {
	archiveDir := filepath.Join(filepath.Dir(s.path), ArchiveDirName)
	entries, err := os.ReadDir(archiveDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("journal: failed to read archive directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "journal-") && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	// Zero-padded sequence ranges in the names make lexicographic order
	// equal to sequence order.
	sort.Strings(names)

	var all []Event
	for _, name := range names {
		events, err := readEventsFile(filepath.Join(archiveDir, name), 0)
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}

	live, err := s.ReadAll()
	if err != nil {
		return nil, err
	}
	return append(all, live...), nil
}
