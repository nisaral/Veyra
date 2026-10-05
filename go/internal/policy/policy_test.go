package policy

import (
	"testing"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

func cand(id string, t veyrav1.ActionType) *veyrav1.ActionCandidate {
	return &veyrav1.ActionCandidate{Id: id, Type: t, ExpectedSuccess: 0.6, EstCostUsd: 0.001}
}

func TestFilterDropsSwitchWhenNoAlternative(t *testing.T) {
	cands := []*veyrav1.ActionCandidate{
		cand("model_call:cheap", veyrav1.ActionType_MODEL_CALL),
		cand("switch_harness:langgraph", veyrav1.ActionType_SWITCH_HARNESS),
	}
	ctx := DefaultContext()
	ctx.HasOtherHarness = false
	got := Filter(cands, ctx)
	if len(got.Allowed) != 1 || got.Allowed[0].Id != "model_call:cheap" {
		t.Fatalf("expected only model_call:cheap, got %+v", ids(got.Allowed))
	}
	if got.Dropped["switch_harness:langgraph"] == "" {
		t.Fatal("expected a drop reason for the switch candidate")
	}
}

func TestFilterDropsSwitchNearBudgetLimit(t *testing.T) {
	cands := []*veyrav1.ActionCandidate{cand("switch_harness:langgraph", veyrav1.ActionType_SWITCH_HARNESS)}
	ctx := DefaultContext()
	ctx.HasOtherHarness = true
	ctx.Fraction = 0.80
	got := Filter(cands, ctx)
	if len(got.Allowed) != 0 {
		t.Fatalf("switch should be blocked above the soft limit, got %+v", ids(got.Allowed))
	}
}

func TestFilterCapsRetryStreak(t *testing.T) {
	cands := []*veyrav1.ActionCandidate{cand("retry", veyrav1.ActionType_RETRY)}
	ctx := DefaultContext()
	ctx.RetriesInARow = 2
	got := Filter(cands, ctx)
	if len(got.Allowed) != 0 {
		t.Fatal("retry should be blocked after the streak cap")
	}
}

func TestFilterDropsMissingPermission(t *testing.T) {
	c := cand("tool_call:shell", veyrav1.ActionType_TOOL_CALL)
	c.RequiredPermissions = []string{"shell"}
	got := Filter([]*veyrav1.ActionCandidate{c}, DefaultContext())
	if len(got.Allowed) != 0 {
		t.Fatal("candidate requiring an ungranted permission must be dropped")
	}
}

func TestTerminateBlockedEarly(t *testing.T) {
	cands := []*veyrav1.ActionCandidate{cand("terminate", veyrav1.ActionType_TERMINATE)}
	ctx := DefaultContext()
	ctx.Step = 1
	if got := Filter(cands, ctx); len(got.Allowed) != 0 {
		t.Fatal("terminate must be blocked before the minimum step count")
	}
	ctx.Step = 5
	if got := Filter(cands, ctx); len(got.Allowed) != 1 {
		t.Fatal("terminate must be allowed later in the run")
	}
}

func TestFixedHarnessOnlyIsTheControl(t *testing.T) {
	cands := []*veyrav1.ActionCandidate{
		cand("model_call:cheap", veyrav1.ActionType_MODEL_CALL),
		cand("model_call:strong", veyrav1.ActionType_MODEL_CALL),
		cand("verify", veyrav1.ActionType_VERIFY),
		cand("switch_harness:langgraph", veyrav1.ActionType_SWITCH_HARNESS),
		cand("terminate", veyrav1.ActionType_TERMINATE),
	}
	got := FixedHarnessOnly(cands)
	if len(got) != 2 {
		t.Fatalf("fixed control must expose exactly 2 candidates, got %+v", ids(got))
	}
}

func ids(cs []*veyrav1.ActionCandidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Id)
	}
	return out
}