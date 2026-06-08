package acp

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SessionManager tracks task-to-session mapping for warm/novice session behavior.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*sessionState

	counter atomic.Uint64
}

type sessionState struct {
	sessionID  string
	lastUsedAt time.Time
	createdAt  time.Time
	runs       int
}

// NewSessionManager creates an isolated ACP session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*sessionState)}
}

// Start returns a session id for the task and whether it is a warm session.
func (m *SessionManager) Start(taskID string) (sessionID string, warm bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state := m.sessions[taskID]; state != nil {
		state.lastUsedAt = time.Now().UTC()
		state.runs++
		return state.sessionID, true
	}

	m.counter.Add(1)
	sessionID = fmt.Sprintf("acp-%d-%d", time.Now().UTC().UnixNano(), m.counter.Load())
	now := time.Now().UTC()
	m.sessions[taskID] = &sessionState{
		sessionID:  sessionID,
		lastUsedAt: now,
		createdAt:  now,
		runs:       1,
	}
	return sessionID, false
}

// End removes the task-to-session mapping, so the next Start for this task is cold.
func (m *SessionManager) End(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, taskID)
}

// Snapshot returns the active session state used for tests and dashboards.
type SessionSnapshot struct {
	TaskID    string
	SessionID string
	CreatedAt time.Time
	LastUsed  time.Time
	Runs      int
}

// List returns a deterministic list of active sessions.
func (m *SessionManager) List() []SessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) == 0 {
		return nil
	}

	result := make([]SessionSnapshot, 0, len(m.sessions))
	for taskID, state := range m.sessions {
		result = append(result, SessionSnapshot{
			TaskID:    taskID,
			SessionID: state.sessionID,
			CreatedAt: state.createdAt,
			LastUsed:  state.lastUsedAt,
			Runs:      state.runs,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TaskID < result[j].TaskID
	})
	return result
}
