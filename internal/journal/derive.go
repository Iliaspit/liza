package journal

import (
	"github.com/liza-mas/liza/internal/models"
)

// Event type names emitted by Derive. Task-history-backed events use the
// history vocabulary prefixed with "task." (e.g. "task.claimed").
const (
	EventTaskCreated       = "task.created"
	EventTaskRemoved       = "task.removed"
	EventTaskStatusChanged = "task.status_changed"

	EventAgentRegistered    = "agent.registered"
	EventAgentUnregistered  = "agent.unregistered"
	EventAgentStatusChanged = "agent.status_changed"

	EventClaimGranted  = "claim.granted"
	EventClaimReleased = "claim.released"

	EventAnomalyLogged    = "anomaly.logged"
	EventDiscoveryLogged  = "discovery.logged"
	EventSpecChanged      = "spec.changed"
	EventHumanNoteAdded   = "human_note.added"
	EventSprintAdvanced   = "sprint.advanced"
	EventSprintStatus     = "sprint.status_changed"
	EventCircuitBreaker   = "circuit_breaker.status_changed"
	EventGoalStatus       = "goal.status_changed"
	EventSystemMode       = "system.mode_changed"
	EventStateModified    = "state.modified"
	taskHistoryEventScope = "task."
)

// Derive computes the journal events implied by a state transition. It is a
// pure function over (before, after); the shadow-journal wiring in db calls
// it inside the write lock so derived events serialize with the state write.
//
// Derivation sources, in order of fidelity:
//  1. New task history entries (the system's existing, proven event log).
//  2. Structural diffs history does not carry: task creation/removal, status
//     changes, agent registry changes, appended anomalies/discoveries/notes,
//     sprint, circuit breaker, and goal transitions.
func Derive(before, after *models.State) []Event {
	var events []Event

	beforeTasks := make(map[string]*models.Task, len(before.Tasks))
	for i := range before.Tasks {
		beforeTasks[before.Tasks[i].ID] = &before.Tasks[i]
	}
	afterTasks := make(map[string]*models.Task, len(after.Tasks))

	for i := range after.Tasks {
		task := &after.Tasks[i]
		afterTasks[task.ID] = task
		prev := beforeTasks[task.ID]

		if prev == nil {
			events = append(events, Event{
				Type: EventTaskCreated,
				Task: task.ID,
				Fields: map[string]any{
					"status":    string(task.Status),
					"type":      string(task.Type),
					"role_pair": task.RolePair,
				},
			})
		}

		events = append(events, deriveHistoryEvents(prev, task)...)

		prevStatus := models.TaskStatus("")
		if prev != nil {
			prevStatus = prev.Status
		}
		if prev != nil && prevStatus != task.Status {
			events = append(events, Event{
				Type: EventTaskStatusChanged,
				Task: task.ID,
				Fields: map[string]any{
					"from": string(prevStatus),
					"to":   string(task.Status),
				},
			})
		}
	}

	for id := range beforeTasks {
		if _, ok := afterTasks[id]; !ok {
			events = append(events, Event{Type: EventTaskRemoved, Task: id})
		}
	}

	events = append(events, deriveAgentEvents(before, after)...)
	events = append(events, deriveClaimEvents(before, after)...)
	events = append(events, deriveAppendOnlyEvents(before, after)...)
	events = append(events, deriveSystemEvents(before, after)...)

	return events
}

// deriveClaimEvents diffs first-class claim records keyed by TaskID+Kind.
// Pure ExpiresAt (and GrantedAt) changes are lease renewals — deliberate
// noise that emits nothing. A claim whose AgentID changed under the same key
// emits a release for the old holder followed by a grant for the new one.
func deriveClaimEvents(before, after *models.State) []Event {
	type claimKey struct {
		taskID string
		kind   string
	}

	beforeClaims := make(map[claimKey]models.Claim, len(before.Claims))
	for _, c := range before.Claims {
		beforeClaims[claimKey{c.TaskID, c.Kind}] = c
	}

	var events []Event
	afterKeys := make(map[claimKey]bool, len(after.Claims))
	for _, c := range after.Claims {
		k := claimKey{c.TaskID, c.Kind}
		afterKeys[k] = true
		prev, existed := beforeClaims[k]
		if existed && prev.AgentID == c.AgentID {
			continue
		}
		if existed {
			events = append(events, claimEvent(EventClaimReleased, prev))
		}
		events = append(events, claimEvent(EventClaimGranted, c))
	}
	for _, c := range before.Claims {
		if !afterKeys[claimKey{c.TaskID, c.Kind}] {
			events = append(events, claimEvent(EventClaimReleased, c))
		}
	}
	return events
}

func claimEvent(eventType string, c models.Claim) Event {
	return Event{
		Type:  eventType,
		Task:  c.TaskID,
		Agent: c.AgentID,
		Fields: map[string]any{
			"kind": c.Kind,
		},
	}
}

func deriveHistoryEvents(prev, task *models.Task) []Event {
	start := 0
	if prev != nil {
		start = len(prev.History)
	}
	if start >= len(task.History) {
		return nil
	}

	var events []Event
	for _, entry := range task.History[start:] {
		ev := Event{
			Type: taskHistoryEventScope + entry.Event,
			Time: entry.Time,
			Task: task.ID,
		}
		if entry.Agent != nil {
			ev.Agent = *entry.Agent
		}
		fields := map[string]any{}
		if entry.Reason != nil {
			fields["reason"] = *entry.Reason
		}
		if entry.Commit != nil {
			fields["commit"] = *entry.Commit
		}
		if entry.Note != nil {
			fields["note"] = *entry.Note
		}
		if entry.PreviousAssignee != nil {
			fields["previous_assignee"] = *entry.PreviousAssignee
		}
		if len(fields) > 0 {
			ev.Fields = fields
		}
		events = append(events, ev)
	}
	return events
}

func deriveAgentEvents(before, after *models.State) []Event {
	var events []Event
	for id, agent := range after.Agents {
		prev, existed := before.Agents[id]
		if !existed {
			events = append(events, Event{
				Type:  EventAgentRegistered,
				Agent: id,
				Fields: map[string]any{
					"role":     agent.Role,
					"provider": agent.Provider,
				},
			})
			continue
		}
		if prev.Status != agent.Status {
			events = append(events, Event{
				Type:  EventAgentStatusChanged,
				Agent: id,
				Fields: map[string]any{
					"from": string(prev.Status),
					"to":   string(agent.Status),
				},
			})
		}
	}
	for id := range before.Agents {
		if _, ok := after.Agents[id]; !ok {
			events = append(events, Event{Type: EventAgentUnregistered, Agent: id})
		}
	}
	return events
}

func deriveAppendOnlyEvents(before, after *models.State) []Event {
	var events []Event

	for _, a := range tailOf(before.Anomalies, after.Anomalies) {
		events = append(events, Event{
			Type:  EventAnomalyLogged,
			Time:  a.Timestamp,
			Task:  a.Task,
			Agent: a.Reporter,
			Fields: map[string]any{
				"anomaly_type": a.Type,
			},
		})
	}
	for _, d := range tailOf(before.Discovered, after.Discovered) {
		events = append(events, Event{
			Type:  EventDiscoveryLogged,
			Time:  d.Created,
			Agent: d.By,
			Fields: map[string]any{
				"id":       d.ID,
				"severity": d.Severity,
				"urgency":  d.Urgency,
			},
		})
	}
	for _, sc := range tailOf(before.SpecChanges, after.SpecChanges) {
		events = append(events, Event{
			Type: EventSpecChanged,
			Time: sc.Timestamp,
			Fields: map[string]any{
				"spec":         sc.Spec,
				"triggered_by": sc.TriggeredBy,
			},
		})
	}
	for _, n := range tailOf(before.HumanNotes, after.HumanNotes) {
		events = append(events, Event{
			Type: EventHumanNoteAdded,
			Time: n.Timestamp,
			Fields: map[string]any{
				"for":     n.For,
				"message": n.Message,
			},
		})
	}
	return events
}

func deriveSystemEvents(before, after *models.State) []Event {
	var events []Event
	if before.Sprint.Number != after.Sprint.Number {
		events = append(events, Event{
			Type: EventSprintAdvanced,
			Fields: map[string]any{
				"from": before.Sprint.Number,
				"to":   after.Sprint.Number,
			},
		})
	}
	if before.Sprint.Status != after.Sprint.Status {
		events = append(events, Event{
			Type: EventSprintStatus,
			Fields: map[string]any{
				"from": string(before.Sprint.Status),
				"to":   string(after.Sprint.Status),
			},
		})
	}
	if before.CircuitBreaker.Status != after.CircuitBreaker.Status {
		events = append(events, Event{
			Type: EventCircuitBreaker,
			Fields: map[string]any{
				"from": before.CircuitBreaker.Status,
				"to":   after.CircuitBreaker.Status,
			},
		})
	}
	if before.Goal.Status != after.Goal.Status {
		events = append(events, Event{
			Type: EventGoalStatus,
			Fields: map[string]any{
				"from": string(before.Goal.Status),
				"to":   string(after.Goal.Status),
			},
		})
	}
	if before.Config.Mode != after.Config.Mode {
		fields := map[string]any{
			"from": string(before.Config.Mode),
			"to":   string(after.Config.Mode),
		}
		if after.Config.ModeChangedBy != nil {
			fields["changed_by"] = *after.Config.ModeChangedBy
		}
		events = append(events, Event{Type: EventSystemMode, Fields: fields})
	}
	return events
}

// tailOf returns the entries appended to an append-only slice.
func tailOf[T any](before, after []T) []T {
	if len(after) <= len(before) {
		return nil
	}
	return after[len(before):]
}
