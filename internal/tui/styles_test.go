package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

func TestStatusColor_ExactMatch(t *testing.T) {
	tests := []struct {
		status string
		want   lipgloss.Color
	}{
		// Active work
		{"WORKING", ColorActive},
		{"RUNNING", ColorActive},
		// Planning
		{"PLANNING", ColorPlanning},
		{"STARTING", ColorPlanning},
		{"PAUSED", ColorPlanning},
		// Review
		{"REVIEWING", ColorReview},
		// Idle/waiting
		{"IDLE", ColorIdle},
		{"WAITING", ColorIdle},
		// Handoff
		{"HANDOFF", ColorHandoff},
		{"CHECKPOINT", ColorHandoff},
		// Approved/done
		{"MERGED", ColorApproved},
		// Rejected/blocked
		{"BLOCKED", ColorRejected},
		{"INTEGRATION_FAILED", ColorRejected},
		{"STOPPED", ColorRejected},
		// Terminal
		{"ABANDONED", ColorTerminal},
		{"SUPERSEDED", ColorTerminal},
		// Bare draft
		{"DRAFT", ColorBareDraft},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusColor(tt.status)
			if got != tt.want {
				t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusColor_SuffixMatch(t *testing.T) {
	tests := []struct {
		status string
		want   lipgloss.Color
	}{
		// *_REJECTED → red
		{"CODE_REJECTED", ColorRejected},
		{"PLAN_REJECTED", ColorRejected},
		// *_APPROVED → green
		{"CODE_APPROVED", ColorApproved},
		{"PLAN_APPROVED", ColorApproved},
		// *_PARTIALLY_APPROVED → green dim
		{"CODE_PARTIALLY_APPROVED", ColorPartialApproved},
		// *_PLANNING → cyan (active work)
		{"CODE_PLANNING", ColorActive},
		{"US_PLANNING", ColorActive},
		// *_TO_REVIEW → blue
		{"CODE_TO_REVIEW", ColorToReview},
		// *_READY_FOR_REVIEW → dim blue (legacy compatibility)
		{"CODE_READY_FOR_REVIEW", ColorToReview},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusColor(tt.status)
			if got != tt.want {
				t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusColor_PrefixMatch(t *testing.T) {
	tests := []struct {
		status string
		want   lipgloss.Color
	}{
		// IMPLEMENTING_* → cyan
		{"IMPLEMENTING_CODE", ColorActive},
		{"IMPLEMENTING_PLAN", ColorActive},
		// REVIEWING_* → blue
		{"REVIEWING_CODE", ColorReview},
		{"REVIEWING_PLAN", ColorReview},
		// DRAFT_* (qualified) → dim white
		{"DRAFT_CODING_PLAN", ColorBareDraft},
		{"DRAFT_EPIC_PLAN", ColorBareDraft},
		{"DRAFT_CODE", ColorBareDraft},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusColor(tt.status)
			if got != tt.want {
				t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusColor_QualifiedDraftVsBareDraft(t *testing.T) {
	// DRAFT (unqualified) → dim white
	got := StatusColor("DRAFT")
	if got != ColorBareDraft {
		t.Errorf("StatusColor(\"DRAFT\") = %q, want %q (bare draft / dim white)", got, ColorBareDraft)
	}

	// DRAFT_CODING_PLAN (qualified) → dim white
	got = StatusColor("DRAFT_CODING_PLAN")
	if got != ColorBareDraft {
		t.Errorf("StatusColor(\"DRAFT_CODING_PLAN\") = %q, want %q (bare draft / dim white)", got, ColorBareDraft)
	}
}

func TestStatusColor_Fallback(t *testing.T) {
	got := StatusColor("UNKNOWN_STATUS_XYZ")
	if got != ColorFallback {
		t.Errorf("StatusColor(\"UNKNOWN_STATUS_XYZ\") = %q, want %q (fallback / white)", got, ColorFallback)
	}

	got = StatusColor("")
	if got != ColorFallback {
		t.Errorf("StatusColor(\"\") = %q, want %q (fallback / white)", got, ColorFallback)
	}
}

func TestTaskStatusColor_PipelineCategoriesOverrideNamePatterns(t *testing.T) {
	tests := []struct {
		status   models.TaskStatus
		category pipeline.StateCategory
		want     lipgloss.Color
	}{
		{"DRAFT_ARCHITECTURE", pipeline.StateCategoryInitial, ColorBareDraft},
		{"ARCHITECTING", pipeline.StateCategoryExecuting, ColorActive},
		{"CODE_PLANNING", pipeline.StateCategoryExecuting, ColorActive},
		{"WRITING_US", pipeline.StateCategoryExecuting, ColorActive},
		{"ANALYZING_INTEGRATION", pipeline.StateCategoryExecuting, ColorActive},
		{"ARCHITECTURE_TO_REVIEW", pipeline.StateCategorySubmitted, ColorToReview},
		{"REVIEWING_CODE_2", pipeline.StateCategoryReviewing2, ColorReview},
		{"CODE_PARTIALLY_APPROVED", pipeline.StateCategoryPartiallyApproved, ColorPartialApproved},
		{"INTEGRATION_ANALYSIS_CLEAN", pipeline.StateCategoryClean, ColorApproved},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := TaskStatusColor(tt.status, map[models.TaskStatus]pipeline.StateCategory{tt.status: tt.category})
			if got != tt.want {
				t.Errorf("TaskStatusColor(%q, %q) = %q, want %q", tt.status, tt.category, got, tt.want)
			}
		})
	}
}

func TestTaskStatusColor_FallbackKeepsLegacyReadyForReview(t *testing.T) {
	got := TaskStatusColor(models.TaskStatus("CODE_READY_FOR_REVIEW"), nil)
	if got != ColorToReview {
		t.Errorf("TaskStatusColor(CODE_READY_FOR_REVIEW, nil) = %q, want %q", got, ColorToReview)
	}
}

func TestStatusDot_ActiveStatuses(t *testing.T) {
	active := []string{"WORKING", "IMPLEMENTING_CODE", "REVIEWING", "PLANNING", "STARTING", "HANDOFF"}
	for _, s := range active {
		got := StatusDot(s)
		if got != "●" {
			t.Errorf("StatusDot(%q) = %q, want \"●\" (filled dot for active)", s, got)
		}
	}
}

func TestStatusDot_IdleStatuses(t *testing.T) {
	idle := []string{"IDLE", "WAITING"}
	for _, s := range idle {
		got := StatusDot(s)
		if got != "○" {
			t.Errorf("StatusDot(%q) = %q, want \"○\" (hollow dot for idle)", s, got)
		}
	}
}

func TestNewStyles_ReturnsPopulatedStruct(t *testing.T) {
	s := NewStyles(120)

	// Verify key styles are not zero-value (check that they have been set by verifying render produces output)
	if s.HeaderBar.GetWidth() != 120 {
		t.Errorf("HeaderBar width = %d, want 120", s.HeaderBar.GetWidth())
	}
	if s.FooterBar.GetWidth() != 120 {
		t.Errorf("FooterBar width = %d, want 120", s.FooterBar.GetWidth())
	}
	// Bordered panels use width-2 so total rendered width (content + border) == terminal width
	if s.AgentPanel.GetWidth() != 118 {
		t.Errorf("AgentPanel width = %d, want 118 (120-2 for border)", s.AgentPanel.GetWidth())
	}
	if s.TaskPanel.GetWidth() != 118 {
		t.Errorf("TaskPanel width = %d, want 118 (120-2 for border)", s.TaskPanel.GetWidth())
	}
	if s.ActivityPanel.GetWidth() != 118 {
		t.Errorf("ActivityPanel width = %d, want 118 (120-2 for border)", s.ActivityPanel.GetWidth())
	}
}

func TestNewStyles_AdaptsToWidth(t *testing.T) {
	narrow := NewStyles(80)
	wide := NewStyles(160)

	if narrow.HeaderBar.GetWidth() != 80 {
		t.Errorf("narrow HeaderBar width = %d, want 80", narrow.HeaderBar.GetWidth())
	}
	if wide.HeaderBar.GetWidth() != 160 {
		t.Errorf("wide HeaderBar width = %d, want 160", wide.HeaderBar.GetWidth())
	}
}

func TestColorConstants_AllDefined(t *testing.T) {
	// Verify all 12 semantic color constants are non-empty
	colors := map[string]lipgloss.Color{
		"ColorActive":          ColorActive,
		"ColorPlanning":        ColorPlanning,
		"ColorReview":          ColorReview,
		"ColorToReview":        ColorToReview,
		"ColorIdle":            ColorIdle,
		"ColorHandoff":         ColorHandoff,
		"ColorApproved":        ColorApproved,
		"ColorPartialApproved": ColorPartialApproved,
		"ColorRejected":        ColorRejected,
		"ColorTerminal":        ColorTerminal,
		"ColorBareDraft":       ColorBareDraft,
		"ColorFallback":        ColorFallback,
	}

	for name, c := range colors {
		if string(c) == "" {
			t.Errorf("%s is empty", name)
		}
	}

	if len(colors) != 12 {
		t.Errorf("expected 12 color constants, got %d", len(colors))
	}
}
