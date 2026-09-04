package prompts

import (
	"strings"
	"testing"
)

func TestPlanningAndReviewPromptsRenderDependencyContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role     string
		sections []string
	}{
		{role: "code-planner", sections: []string{"code-planner-tools", "task-decomposition", "implementation-phase"}},
		{role: "architect", sections: []string{"architect-tools", "implementation-phase"}},
		{role: "epic-planner", sections: []string{"epic-planner-tools", "implementation-phase"}},
		{role: "integration-analyst", sections: []string{"integration-analyst-tools"}},
		{role: "code-plan-reviewer", sections: []string{"review-instructions"}},
		{role: "architecture-reviewer", sections: []string{"review-instructions"}},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()
			data := &RoleContextData{Role: tt.role, TaskID: "plan-1", AgentID: tt.role + "-1", GoalSlug: "goal"}
			rendered, err := BuildRoleContext(tt.role, tt.sections, data)
			if err != nil {
				t.Fatalf("BuildRoleContext() error: %v", err)
			}
			for _, want := range []string{"dependency_contracts", "before_start", "before_approval_merge", "severity"} {
				if !strings.Contains(rendered, want) {
					t.Errorf("rendered prompt missing %q", want)
				}
			}
		})
	}
}

func TestGraphReplanWakePromptKeepsControllerReadOnly(t *testing.T) {
	t.Parallel()

	rendered, err := buildInstructionsForWakeTrigger("GRAPH_REPLAN_REQUEST", "orchestrator-1", wakeTemplateData{
		GraphReplanRequestID:  "graph-replan-abc",
		GraphReplanDiagnostic: "circular dependency detected: task-a -> task-b -> task-a",
	}, nil)
	if err != nil {
		t.Fatalf("buildInstructionsForWakeTrigger() error: %v", err)
	}
	for _, want := range []string{
		"claim-graph-replan graph-replan-abc",
		"complete-graph-replan graph-replan-abc",
		"do not retry the unchanged operation",
		"refresh-graph-replan",
		"use only Lisa-native task operations",
		"Refuse product, scope, or acceptance changes",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered wake prompt missing %q", want)
		}
	}
}
