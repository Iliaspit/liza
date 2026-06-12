package scheduler

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// embeddedResolver loads the production pipeline config so tests exercise the
// real role-pair/status wiring rather than a hand-rolled fixture.
func embeddedResolver(t *testing.T) *pipeline.Resolver {
	t.Helper()
	cfg, err := pipeline.LoadEmbeddedReference()
	if err != nil {
		t.Fatalf("LoadEmbeddedReference: %v", err)
	}
	return pipeline.NewResolver(cfg)
}

// status helpers resolve canonical coding-pair statuses from the config so the
// test stays correct if the embedded status names change.
func codingStatus(t *testing.T, r *pipeline.Resolver, which string) models.TaskStatus {
	t.Helper()
	var (
		s   models.TaskStatus
		err error
	)
	switch which {
	case "initial":
		s, err = r.InitialStatus("coding-pair")
	case "submitted":
		s, err = r.SubmittedStatus("coding-pair")
	case "approved":
		s, err = r.ApprovedStatus("coding-pair")
	default:
		t.Fatalf("unknown status key %q", which)
	}
	if err != nil {
		t.Fatalf("resolve %s status: %v", which, err)
	}
	return s
}

func TestCompute_ClassifiesDoerReviewMerge(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()

	reviewCommit := "rev123"
	state := &models.State{
		Tasks: []models.Task{
			{ID: "t-doer", RolePair: "coding-pair", Status: codingStatus(t, r, "initial")},
			{ID: "t-review", RolePair: "coding-pair", Status: codingStatus(t, r, "submitted"), ReviewCommit: &reviewCommit},
			{ID: "t-merge", RolePair: "coding-pair", Status: codingStatus(t, r, "approved"), ReviewCommit: &reviewCommit},
		},
		Agents: map[string]models.Agent{},
	}

	plan := Compute(state, r, now)
	counts := plan.Counts()
	if counts[WorkDoer] != 1 || counts[WorkReview] != 1 || counts[WorkMerge] != 1 {
		t.Fatalf("unexpected counts: %+v (items=%+v)", counts, plan.Items)
	}

	byTask := map[string]WorkItem{}
	for _, it := range plan.Items {
		byTask[it.TaskID] = it
	}
	if byTask["t-doer"].Kind != WorkDoer {
		t.Errorf("t-doer = %s, want doer", byTask["t-doer"].Kind)
	}
	if byTask["t-review"].Kind != WorkReview {
		t.Errorf("t-review = %s, want review", byTask["t-review"].Kind)
	}
	if byTask["t-merge"].Kind != WorkMerge {
		t.Errorf("t-merge = %s, want merge", byTask["t-merge"].Kind)
	}

	// Roles are populated for doer/review, empty for merge.
	if byTask["t-doer"].Role == "" || byTask["t-review"].Role == "" {
		t.Errorf("expected roles on doer/review items: %+v", plan.Items)
	}
	if byTask["t-merge"].Role != "" {
		t.Errorf("merge item should carry no role, got %q", byTask["t-merge"].Role)
	}

	// Deterministic ordering: merge before review before doer.
	if plan.Items[0].Kind != WorkMerge || plan.Items[len(plan.Items)-1].Kind != WorkDoer {
		t.Errorf("items not ordered merge→review→doer: %+v", plan.Items)
	}
}

func TestCompute_ActivelyClaimedTaskIsNotRunnable(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()

	// A task being implemented (assigned, live lease) must not surface as work.
	agent := "coder-1"
	worktree := ".worktrees/t1"
	base := "abc"
	future := now.Add(30 * time.Minute)
	executing, err := r.ExecutingStatus("coding-pair")
	if err != nil {
		t.Fatalf("executing status: %v", err)
	}
	state := &models.State{
		Tasks: []models.Task{{
			ID: "t1", RolePair: "coding-pair", Status: executing,
			AssignedTo: &agent, Worktree: &worktree, BaseCommit: &base, LeaseExpires: &future,
		}},
		Agents: map[string]models.Agent{},
	}

	plan := Compute(state, r, now)
	if len(plan.Items) != 0 {
		t.Fatalf("actively-claimed task produced work items: %+v", plan.Items)
	}
}

func TestCompute_ExpiredClaimSurfacesReclaim(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()
	expired := now.Add(-models.LeaseExpiryGracePeriod - time.Minute)

	state := &models.State{
		Tasks: []models.Task{{ID: "t1", RolePair: "coding-pair", Status: codingStatus(t, r, "initial")}},
		Claims: []models.Claim{
			{TaskID: "t1", AgentID: "coder-9", Kind: models.ClaimKindDoer, ExpiresAt: &expired},
		},
		Agents: map[string]models.Agent{},
	}

	reclaims := Compute(state, r, now).ByKind(WorkReclaim)
	if len(reclaims) != 1 || reclaims[0].AgentID != "coder-9" || reclaims[0].TaskID != "t1" {
		t.Fatalf("expected one reclaim for coder-9 on t1, got %+v", reclaims)
	}
}

func TestCompute_FreshClaimDoesNotReclaim(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()
	future := now.Add(30 * time.Minute)

	state := &models.State{
		Tasks:  []models.Task{{ID: "t1", RolePair: "coding-pair", Status: codingStatus(t, r, "initial")}},
		Claims: []models.Claim{{TaskID: "t1", AgentID: "coder-9", Kind: models.ClaimKindDoer, ExpiresAt: &future}},
		Agents: map[string]models.Agent{},
	}
	if got := Compute(state, r, now).ByKind(WorkReclaim); len(got) != 0 {
		t.Fatalf("fresh claim produced reclaim: %+v", got)
	}
}

func TestCompute_NilInputsAndNoRolePair(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()

	if got := Compute(nil, r, now); len(got.Items) != 0 {
		t.Errorf("nil state should yield empty plan, got %+v", got)
	}
	if got := Compute(&models.State{}, nil, now); len(got.Items) != 0 {
		t.Errorf("nil resolver should yield empty plan, got %+v", got)
	}
	// Legacy task without role_pair is skipped (not classifiable).
	state := &models.State{Tasks: []models.Task{{ID: "legacy", Status: "READY"}}, Agents: map[string]models.Agent{}}
	if got := Compute(state, r, now); len(got.Items) != 0 {
		t.Errorf("role-pair-less task should be skipped, got %+v", got)
	}
}

func TestCompute_Deterministic(t *testing.T) {
	r := embeddedResolver(t)
	now := time.Now().UTC()
	reviewCommit := "rev"
	state := &models.State{
		Tasks: []models.Task{
			{ID: "b", RolePair: "coding-pair", Status: codingStatus(t, r, "initial")},
			{ID: "a", RolePair: "coding-pair", Status: codingStatus(t, r, "initial")},
			{ID: "m", RolePair: "coding-pair", Status: codingStatus(t, r, "approved"), ReviewCommit: &reviewCommit},
		},
		Agents: map[string]models.Agent{},
	}
	first := Compute(state, r, now)
	second := Compute(state, r, now)
	if len(first.Items) != len(second.Items) {
		t.Fatalf("non-deterministic length")
	}
	for i := range first.Items {
		if first.Items[i] != second.Items[i] {
			t.Fatalf("non-deterministic at %d: %+v vs %+v", i, first.Items[i], second.Items[i])
		}
	}
	// Within doer kind, task IDs sorted ascending: a before b.
	doers := first.ByKind(WorkDoer)
	if len(doers) != 2 || doers[0].TaskID != "a" || doers[1].TaskID != "b" {
		t.Fatalf("doer items not sorted by task id: %+v", doers)
	}
}
