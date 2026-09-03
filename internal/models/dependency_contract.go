package models

import "time"

// DependencyContractVersion is written on new runs so legacy state remains
// readable while new planning output is held to the typed dependency contract.
const DependencyContractVersion = 1

type DependencyGate string

const (
	DependencyGateBeforeStart    DependencyGate = "before_start"
	DependencyGateBeforeApproval DependencyGate = "before_approval_merge"
	DependencyGateAdvisory       DependencyGate = "advisory"
)

func (g DependencyGate) IsValid() bool {
	return g == DependencyGateBeforeStart || g == DependencyGateBeforeApproval || g == DependencyGateAdvisory
}

type DependencySeverity string

const (
	// Critical means the declared consumer contract cannot be satisfied safely.
	DependencySeverityCritical DependencySeverity = "critical"
	// High means work must stop at the declared lifecycle gate.
	DependencySeverityHigh DependencySeverity = "high"
	// Medium means work may proceed only up to the declared lifecycle gate.
	DependencySeverityMedium DependencySeverity = "medium"
	// Low means the dependency is advisory and does not gate lifecycle progress.
	DependencySeverityLow DependencySeverity = "low"
)

func (s DependencySeverity) IsValid() bool {
	return s == DependencySeverityCritical || s == DependencySeverityHigh || s == DependencySeverityMedium || s == DependencySeverityLow
}

// DependencyContract describes why one task consumes another task's output.
// ProviderOutput is used only in planner output and is resolved to the
// deterministic sibling child task when the output is materialized.
type DependencyContract struct {
	ProviderTask   string             `yaml:"provider_task,omitempty" json:"provider_task,omitempty"`
	ProviderOutput *int               `yaml:"provider_output,omitempty" json:"provider_output,omitempty"`
	Purpose        string             `yaml:"purpose" json:"purpose"`
	Gate           DependencyGate     `yaml:"gate" json:"gate"`
	Severity       DependencySeverity `yaml:"severity" json:"severity"`
	Supplies       string             `yaml:"supplies" json:"supplies"`
}

type GraphReplanStatus string

const (
	GraphReplanPending   GraphReplanStatus = "pending"
	GraphReplanRepairing GraphReplanStatus = "repairing"
	GraphReplanCompleted GraphReplanStatus = "completed"
)

// DependencyGraphEdge is the audit representation used to bind a re-plan
// request to the exact graph that was diagnosed.
type DependencyGraphEdge struct {
	Consumer string             `yaml:"consumer" json:"consumer"`
	Provider string             `yaml:"provider" json:"provider"`
	Gate     DependencyGate     `yaml:"gate" json:"gate"`
	Severity DependencySeverity `yaml:"severity" json:"severity"`
	Purpose  string             `yaml:"purpose" json:"purpose"`
	Supplies string             `yaml:"supplies" json:"supplies"`
}

// GraphReplanRequest is an append-only audit record. The controller may create
// it, but only the generation-fenced native orchestrator may claim or complete it.
type GraphReplanRequest struct {
	ID                   string                `yaml:"id" json:"id"`
	RunID                string                `yaml:"run_id" json:"run_id"`
	GraphGeneration      string                `yaml:"graph_generation" json:"graph_generation"`
	ScopeFingerprint     string                `yaml:"scope_fingerprint" json:"scope_fingerprint"`
	CandidateFingerprint string                `yaml:"candidate_fingerprint" json:"candidate_fingerprint"`
	RequestedBy          string                `yaml:"requested_by" json:"requested_by"`
	Reason               string                `yaml:"reason" json:"reason"`
	Diagnostic           string                `yaml:"diagnostic" json:"diagnostic"`
	RequestedAt          time.Time             `yaml:"requested_at" json:"requested_at"`
	Status               GraphReplanStatus     `yaml:"status" json:"status"`
	Orchestrator         *AgentAuthority       `yaml:"orchestrator,omitempty" json:"orchestrator,omitempty"`
	StartedAt            *time.Time            `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt          *time.Time            `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	ResultGeneration     string                `yaml:"result_generation,omitempty" json:"result_generation,omitempty"`
	Diagnosis            string                `yaml:"diagnosis,omitempty" json:"diagnosis,omitempty"`
	GraphChanges         []string              `yaml:"graph_changes,omitempty" json:"graph_changes,omitempty"`
	ValidationResult     string                `yaml:"validation_result,omitempty" json:"validation_result,omitempty"`
	InitialTaskIDs       []string              `yaml:"initial_task_ids,omitempty" json:"initial_task_ids,omitempty"`
	InitialEdges         []DependencyGraphEdge `yaml:"initial_edges,omitempty" json:"initial_edges,omitempty"`
}

func (s *State) OpenGraphReplanRequest() *GraphReplanRequest {
	if s == nil {
		return nil
	}
	for i := range s.GraphReplanRequests {
		if s.GraphReplanRequests[i].Status == GraphReplanPending || s.GraphReplanRequests[i].Status == GraphReplanRepairing {
			return &s.GraphReplanRequests[i]
		}
	}
	return nil
}
