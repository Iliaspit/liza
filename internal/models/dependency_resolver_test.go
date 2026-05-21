package models

import "testing"

func TestDependencyResolver(t *testing.T) {
	tests := []struct {
		name       string
		tasks      []Task
		depID      string
		wantKind   DependencySatisfactionKind
		wantOK     bool
		wantPath   []string
		wantBlocks []string
	}{
		{
			name:     "merged dependency satisfies directly",
			tasks:    []Task{{ID: "dep", Status: TaskStatusMerged}},
			depID:    "dep",
			wantKind: DependencySatisfiedDirect,
			wantOK:   true,
			wantPath: []string{"dep"},
		},
		{
			name:       "pending dependency blocks",
			tasks:      []Task{{ID: "dep", Status: TaskStatusReady}},
			depID:      "dep",
			wantKind:   DependencyUnsatisfiedPending,
			wantBlocks: []string{"dep"},
		},
		{
			name:       "superseded without replacement blocks",
			tasks:      []Task{{ID: "dep", Status: TaskStatusSuperseded}},
			depID:      "dep",
			wantKind:   DependencyUnsatisfiedSuperseded,
			wantBlocks: []string{"dep"},
		},
		{
			name: "superseded with merged replacement satisfies",
			tasks: []Task{
				{ID: "dep", Status: TaskStatusSuperseded, SupersededBy: []string{"replacement"}},
				{ID: "replacement", Status: TaskStatusMerged},
			},
			depID:    "dep",
			wantKind: DependencySatisfiedViaSupersession,
			wantOK:   true,
			wantPath: []string{"dep", "replacement"},
		},
		{
			name: "superseded replacement chain satisfies recursively",
			tasks: []Task{
				{ID: "dep", Status: TaskStatusSuperseded, SupersededBy: []string{"replacement-1"}},
				{ID: "replacement-1", Status: TaskStatusSuperseded, SupersededBy: []string{"replacement-2"}},
				{ID: "replacement-2", Status: TaskStatusMerged},
			},
			depID:    "dep",
			wantKind: DependencySatisfiedViaSupersession,
			wantOK:   true,
			wantPath: []string{"dep", "replacement-1", "replacement-2"},
		},
		{
			name: "split replacements require all branches",
			tasks: []Task{
				{ID: "dep", Status: TaskStatusSuperseded, SupersededBy: []string{"done", "pending"}},
				{ID: "done", Status: TaskStatusMerged},
				{ID: "pending", Status: TaskStatusReady},
			},
			depID:      "dep",
			wantKind:   DependencyUnsatisfiedPendingReplacement,
			wantBlocks: []string{"pending"},
		},
		{
			name: "missing replacement is invalid",
			tasks: []Task{
				{ID: "dep", Status: TaskStatusSuperseded, SupersededBy: []string{"missing"}},
			},
			depID:      "dep",
			wantKind:   DependencyInvalidMissing,
			wantBlocks: []string{"missing"},
		},
		{
			name: "supersession cycle is invalid",
			tasks: []Task{
				{ID: "dep", Status: TaskStatusSuperseded, SupersededBy: []string{"replacement"}},
				{ID: "replacement", Status: TaskStatusSuperseded, SupersededBy: []string{"dep"}},
			},
			depID:      "dep",
			wantKind:   DependencyInvalidCycle,
			wantBlocks: []string{"dep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{Tasks: tt.tasks}
			got := state.ResolveDependency(tt.depID)
			if got.Kind != tt.wantKind {
				t.Fatalf("Kind = %q, want %q; result: %#v", got.Kind, tt.wantKind, got)
			}
			if got.Satisfied() != tt.wantOK {
				t.Fatalf("Satisfied() = %v, want %v; result: %#v", got.Satisfied(), tt.wantOK, got)
			}
			if len(tt.wantPath) > 0 {
				assertStringSliceEqual(t, "Path", got.Path, tt.wantPath)
			}
			if len(tt.wantBlocks) > 0 {
				assertStringSliceEqual(t, "BlockingIDs", got.BlockingIDs, tt.wantBlocks)
			}
		})
	}
}

func TestDependencySatisfactionViaSupersession(t *testing.T) {
	tests := []struct {
		name string
		in   DependencySatisfaction
		want bool
	}{
		{
			name: "direct pending is not via supersession",
			in:   DependencySatisfaction{Kind: DependencyUnsatisfiedPending, Path: []string{"dep"}},
		},
		{
			name: "pending replacement is via supersession",
			in:   DependencySatisfaction{Kind: DependencyUnsatisfiedPendingReplacement, Path: []string{"dep", "replacement"}},
			want: true,
		},
		{
			name: "superseded without replacement is via supersession",
			in:   DependencySatisfaction{Kind: DependencyUnsatisfiedSuperseded, Path: []string{"dep"}},
			want: true,
		},
		{
			name: "missing direct dependency is not via supersession",
			in:   DependencySatisfaction{Kind: DependencyInvalidMissing, Path: []string{"dep"}},
		},
		{
			name: "missing replacement is via supersession",
			in:   DependencySatisfaction{Kind: DependencyInvalidMissing, Path: []string{"dep", "replacement"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.ViaSupersession(); got != tt.want {
				t.Fatalf("ViaSupersession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertStringSliceEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s len = %d, want %d; got %v want %v", name, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q; got %v want %v", name, i, got[i], want[i], got, want)
		}
	}
}
