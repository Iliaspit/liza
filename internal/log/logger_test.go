package log

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestLoggerAppend(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Create logger
	logger := New(logPath)

	// Append first entry
	entry := Entry{
		Agent:  "test-agent",
		Action: "test_action",
		Detail: "test detail",
	}

	if err := logger.Append(entry); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Read log file and verify
	entries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Agent != entry.Agent {
		t.Errorf("Agent = %q, want %q", entries[0].Agent, entry.Agent)
	}
	if entries[0].Action != entry.Action {
		t.Errorf("Action = %q, want %q", entries[0].Action, entry.Action)
	}
	if entries[0].Detail != entry.Detail {
		t.Errorf("Detail = %q, want %q", entries[0].Detail, entry.Detail)
	}
	if entries[0].Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}

	// Verify timestamp is recent
	now := time.Now().UTC()
	diff := now.Sub(entries[0].Timestamp)
	if diff < 0 || diff > 2*time.Second {
		t.Errorf("Timestamp diff = %v, want < 2s", diff)
	}
}

func TestLoggerAppendWithTask(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	taskID := "task-123"
	entry := Entry{
		Agent:  "coder-1",
		Action: "task_claimed",
		Task:   &taskID,
		Detail: "Claimed task",
	}

	if err := logger.Append(entry); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	entries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Task == nil {
		t.Fatal("Task is nil")
	}
	if *entries[0].Task != taskID {
		t.Errorf("Task = %q, want %q", *entries[0].Task, taskID)
	}
}

func TestLoggerAppendMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	// Append multiple entries
	entries := []Entry{
		{Agent: "system", Action: "initialized", Detail: "System started"},
		{Agent: "orchestrator-1", Action: "task_added", Detail: "Added new task"},
		{Agent: "coder-1", Action: "task_claimed", Detail: "Claimed task"},
	}

	for _, entry := range entries {
		if err := logger.Append(entry); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	// Read and verify
	readEntries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(readEntries) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(readEntries))
	}

	for i, expected := range entries {
		if readEntries[i].Agent != expected.Agent {
			t.Errorf("Entry[%d].Agent = %q, want %q", i, readEntries[i].Agent, expected.Agent)
		}
		if readEntries[i].Action != expected.Action {
			t.Errorf("Entry[%d].Action = %q, want %q", i, readEntries[i].Action, expected.Action)
		}
	}
}

func TestLoggerAppendToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Create initial log file
	initialEntries := []Entry{
		{
			Timestamp: time.Now().UTC(),
			Agent:     "system",
			Action:    "initialized",
			Detail:    "Initial entry",
		},
	}

	data, err := yaml.Marshal(initialEntries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Append new entry
	logger := New(logPath)
	newEntry := Entry{
		Agent:  "orchestrator-1",
		Action: "task_added",
		Detail: "New task",
	}

	if err := logger.Append(newEntry); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Verify both entries exist
	entries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify order is preserved
	if entries[0].Agent != "system" {
		t.Errorf("First entry Agent = %q, want %q", entries[0].Agent, "system")
	}
	if entries[1].Agent != "orchestrator-1" {
		t.Errorf("Second entry Agent = %q, want %q", entries[1].Agent, "orchestrator-1")
	}
}

func TestLoggerNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Don't create the file - logger should handle it
	logger := New(logPath)

	entry := Entry{
		Agent:  "system",
		Action: "initialized",
		Detail: "First entry",
	}

	if err := logger.Append(entry); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}

	// Verify entry
	entries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestLoggerConcurrentAppends(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	// Concurrent appends
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			entry := Entry{
				Agent:  "agent-" + string(rune('0'+id)),
				Action: "test_action",
				Detail: "Concurrent test",
			}
			if err := logger.Append(entry); err != nil {
				t.Errorf("Append() error = %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range numGoroutines {
		<-done
	}

	// Verify all entries were written
	entries, err := readLogFile(logPath)
	if err != nil {
		t.Fatalf("readLogFile() error = %v", err)
	}

	if len(entries) != numGoroutines {
		t.Errorf("expected %d entries, got %d", numGoroutines, len(entries))
	}
}

func TestEntryValidation(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{
			name: "valid entry",
			entry: Entry{
				Agent:  "test-agent",
				Action: "test_action",
				Detail: "test detail",
			},
			wantErr: false,
		},
		{
			name: "empty agent",
			entry: Entry{
				Agent:  "",
				Action: "test_action",
				Detail: "test detail",
			},
			wantErr: true,
		},
		{
			name: "empty action",
			entry: Entry{
				Agent:  "test-agent",
				Action: "",
				Detail: "test detail",
			},
			wantErr: true,
		},
		{
			name: "empty detail is ok",
			entry: Entry{
				Agent:  "test-agent",
				Action: "test_action",
				Detail: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoggerWithLockTimeout tests custom lock timeout
func TestLoggerWithLockTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)
	customTimeout := 5 * time.Second

	// Create logger with custom timeout
	customLogger := logger.WithLockTimeout(customTimeout)

	// Verify path is preserved
	if customLogger.logPath != logPath {
		t.Errorf("logPath = %q, want %q", customLogger.logPath, logPath)
	}

	// Verify the custom logger works (functional test instead of field inspection)
	entry := Entry{
		Agent:  "test-agent",
		Action: "test_action",
		Detail: "test detail",
	}
	if err := customLogger.Append(entry); err != nil {
		t.Fatalf("Append with custom timeout failed: %v", err)
	}
}

// TestLoggerRead tests the Read method
func TestLoggerRead(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	// Test reading non-existent file
	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() on non-existent file error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(entries))
	}

	// Append some entries
	entry1 := Entry{
		Agent:  "agent-1",
		Action: "action1",
		Detail: "detail1",
	}
	entry2 := Entry{
		Agent:  "agent-2",
		Action: "action2",
		Detail: "detail2",
	}

	if err := logger.Append(entry1); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := logger.Append(entry2); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Read the entries
	entries, err = logger.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verify entries match
	if entries[0].Agent != entry1.Agent {
		t.Errorf("Entry[0].Agent = %q, want %q", entries[0].Agent, entry1.Agent)
	}
	if entries[1].Agent != entry2.Agent {
		t.Errorf("Entry[1].Agent = %q, want %q", entries[1].Agent, entry2.Agent)
	}
}

// TestLoggerReadEmptyFile tests reading an empty log file
func TestLoggerReadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Create empty file
	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	logger := New(logPath)
	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(entries))
	}
}

// TestLoggerAppendInvalidEntry tests appending an invalid entry
func TestLoggerAppendInvalidEntry(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	// Try to append entry without agent
	entry := Entry{
		Action: "test_action",
		Detail: "test detail",
	}

	err := logger.Append(entry)
	if err == nil {
		t.Error("Expected error appending invalid entry, got nil")
	}
}

// TestLoggerAppendWithPresetTimestamp tests appending with a pre-set timestamp
func TestLoggerAppendWithPresetTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	logger := New(logPath)

	// Create entry with specific timestamp
	customTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	entry := Entry{
		Timestamp: customTime,
		Agent:     "test-agent",
		Action:    "test_action",
		Detail:    "test detail",
	}

	if err := logger.Append(entry); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Read and verify timestamp was preserved
	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if !entries[0].Timestamp.Equal(customTime) {
		t.Errorf("Timestamp = %v, want %v", entries[0].Timestamp, customTime)
	}
}

// TestLoggerReadCorruptedFile tests reading a corrupted log file
func TestLoggerReadCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Write invalid YAML
	if err := os.WriteFile(logPath, []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatal(err)
	}

	logger := New(logPath)
	_, err := logger.Read()
	if err == nil {
		t.Error("Expected error reading corrupted file, got nil")
	}
}

func TestLoggerIgnoresLegacyPIDMetadataWhenLockFree(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")
	lockPath := logPath + ".lock"
	pidPath := logPath + ".lock.pid"
	ownerPath := logPath + ".lock.owner.json"

	if err := os.WriteFile(lockPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("99999999"), 0644); err != nil {
		t.Fatal(err)
	}

	logger := New(logPath).WithLockTimeout(2 * time.Second)

	entry := Entry{
		Agent:  "test-agent",
		Action: "legacy_metadata",
		Detail: "entry after legacy metadata was ignored",
	}

	if err := logger.Append(entry); err != nil {
		t.Fatalf("Append() should succeed when flock is free: %v", err)
	}

	// Verify entry was written
	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Agent != "test-agent" {
		t.Errorf("Agent = %q, want %q", entries[0].Agent, "test-agent")
	}

	legacyPID, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("legacy PID metadata should remain available for diagnostics: %v", err)
	}
	if string(legacyPID) != "99999999" {
		t.Errorf("legacy PID metadata = %q, want unchanged", legacyPID)
	}

	metadataData, err := os.ReadFile(ownerPath)
	if err != nil {
		t.Fatalf("owner metadata was not written: %v", err)
	}
	var metadata struct {
		PID       int    `json:"pid"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatalf("owner metadata is invalid JSON: %v", err)
	}
	if metadata.PID != os.Getpid() {
		t.Errorf("owner metadata PID = %d, want %d", metadata.PID, os.Getpid())
	}
	if metadata.Operation != "append" {
		t.Errorf("owner metadata operation = %q, want append", metadata.Operation)
	}
}

// Helper function to read log file
func readLogFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// TestLoggerGetLastTimestamp tests the GetLastTimestamp method
func TestLoggerGetLastTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")
	logger := New(logPath)

	// Test 1: Empty/non-existent log file
	ts, err := logger.GetLastTimestamp()
	if err != nil {
		t.Fatalf("GetLastTimestamp on empty file returned error: %v", err)
	}
	if !ts.IsZero() {
		t.Errorf("GetLastTimestamp on empty file should return zero time, got: %v", ts)
	}

	// Test 2: Add entries and check last timestamp
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)

	if err := logger.Append(Entry{
		Timestamp: oldTime,
		Agent:     "agent-1",
		Action:    "test-action",
		Detail:    "first entry",
	}); err != nil {
		t.Fatalf("couldn't append first entry: %v", err)
	}

	if err := logger.Append(Entry{
		Timestamp: newTime,
		Agent:     "agent-2",
		Action:    "test-action-2",
		Detail:    "second entry",
	}); err != nil {
		t.Fatalf("couldn't append second entry: %v", err)
	}

	ts, err = logger.GetLastTimestamp()
	if err != nil {
		t.Fatalf("GetLastTimestamp returned error: %v", err)
	}
	if !ts.Equal(newTime) {
		t.Errorf("GetLastTimestamp should return %v, got: %v", newTime, ts)
	}
}

func TestLoggerAppendPreservesExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")
	logger := New(logPath)

	existing := []byte(
		"- timestamp: 2026-01-01T12:00:00Z\n" +
			"  agent: system\n" +
			"  action: initialized\n" +
			"  detail: initial entry\n" +
			"  extra_field: keep-me\n",
	)
	if err := os.WriteFile(logPath, existing, 0644); err != nil {
		t.Fatalf("failed to write existing log: %v", err)
	}

	nextTime := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	if err := logger.Append(Entry{
		Timestamp: nextTime,
		Agent:     "coder-1",
		Action:    "task_claimed",
		Detail:    "claimed task",
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log after append: %v", err)
	}
	if !bytes.HasPrefix(data, existing) {
		t.Fatalf("append rewrote existing content; file no longer starts with original bytes")
	}

	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if !entries[1].Timestamp.Equal(nextTime) {
		t.Errorf("last timestamp = %v, want %v", entries[1].Timestamp, nextTime)
	}
}

func TestLoggerGetLastTimestampTailWindow(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")
	logger := New(logPath)

	prefix := bytes.Repeat([]byte("invalid: [\n"), 10000)
	expected := time.Date(2026, 1, 1, 12, 45, 0, 0, time.UTC)
	tail := []byte(
		"- timestamp: 2026-01-01T12:00:00Z\n" +
			"  agent: system\n" +
			"  action: initialized\n" +
			"  detail: startup\n" +
			"- timestamp: 2026-01-01T12:45:00Z\n" +
			"  agent: coder-1\n" +
			"  action: task_claimed\n" +
			"  detail: claimed task\n",
	)

	if err := os.WriteFile(logPath, append(prefix, tail...), 0644); err != nil {
		t.Fatalf("failed to write log fixture: %v", err)
	}

	ts, err := logger.GetLastTimestamp()
	if err != nil {
		t.Fatalf("GetLastTimestamp returned error: %v", err)
	}
	if !ts.Equal(expected) {
		t.Errorf("GetLastTimestamp should return %v, got %v", expected, ts)
	}
}
