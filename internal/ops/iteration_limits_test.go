package ops

import "testing"

func TestClassifyLimitEscalation_NoCapHit(t *testing.T) {
	t.Parallel()

	_, hit := classifyLimitEscalation(2, 5, 3, 10, 1)
	if hit {
		t.Fatal("expected no escalation when neither cap is reached")
	}
}

func TestClassifyLimitEscalation_Attempt1_ReviewCapHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(5, 5, 3, 10, 1)
	if !hit {
		t.Fatal("expected escalation when review cap is reached")
	}
	if esc.action != LimitActionNewAttempt {
		t.Fatalf("expected LimitActionNewAttempt, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestClassifyLimitEscalation_Attempt1_IterationCapHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(2, 5, 10, 10, 1)
	if !hit {
		t.Fatal("expected escalation when iteration cap is reached")
	}
	if esc.action != LimitActionNewAttempt {
		t.Fatalf("expected LimitActionNewAttempt, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestClassifyLimitEscalation_Attempt1_BothCapsHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(5, 5, 10, 10, 1)
	if !hit {
		t.Fatal("expected escalation when both caps are reached")
	}
	if esc.action != LimitActionNewAttempt {
		t.Fatalf("expected LimitActionNewAttempt, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestClassifyLimitEscalation_Attempt2_ReviewCapHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(5, 5, 3, 10, 2)
	if !hit {
		t.Fatal("expected escalation when review cap is reached")
	}
	if esc.action != LimitActionBlocked {
		t.Fatalf("expected LimitActionBlocked, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestClassifyLimitEscalation_Attempt2_IterationCapHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(2, 5, 10, 10, 2)
	if !hit {
		t.Fatal("expected escalation when iteration cap is reached")
	}
	if esc.action != LimitActionBlocked {
		t.Fatalf("expected LimitActionBlocked, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestClassifyLimitEscalation_Attempt2_BothCapsHit(t *testing.T) {
	t.Parallel()

	esc, hit := classifyLimitEscalation(5, 5, 10, 10, 2)
	if !hit {
		t.Fatal("expected escalation when both caps are reached")
	}
	if esc.action != LimitActionBlocked {
		t.Fatalf("expected LimitActionBlocked, got %d", esc.action)
	}
	if esc.reason == "" {
		t.Fatal("expected non-empty reason")
	}
}
