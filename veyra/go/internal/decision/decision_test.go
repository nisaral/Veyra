package decision

import (
	"testing"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

func TestHeuristicPrefersRecoveryOverTerminationOnFailure(t *testing.T) {
	h := NewHeuristic()
	state := &veyrav1.CommonExecutionState{Failed: true, Step: 3}
	allowed := []*veyrav1.ActionCandidate{
		{Id: "retry", Type: veyrav1.ActionType_RETRY, ExpectedSuccess: 0.45, EstCostUsd: 0.002, Risk: 0.30},
		{Id: "switch_harness:langgraph", Type: veyrav1.ActionType_SWITCH_HARNESS, ExpectedSuccess: 0.72, EstCostUsd: 0.005, Risk: 0.25},
		{Id: "terminate", Type: veyrav1.ActionType_TERMINATE, ExpectedSuccess: 1.0},
	}
	sc := h.Score(state, allowed)
	if sc["terminate"] >= sc["switch_harness:langgraph"] {
		t.Fatalf("terminate (%f) must not beat switching (%f) while recovery is possible",
			sc["terminate"], sc["switch_harness:langgraph"])
	}
}

func TestHeuristicTerminatesWhenDone(t *testing.T) {
	h := NewHeuristic()
	state := &veyrav1.CommonExecutionState{Done: true, Step: 5}
	allowed := []*veyrav1.ActionCandidate{
		{Id: "model_call:cheap", Type: veyrav1.ActionType_MODEL_CALL, ExpectedSuccess: 0.55},
		{Id: "terminate", Type: veyrav1.ActionType_TERMINATE, ExpectedSuccess: 1.0},
	}
	sc := h.Score(state, allowed)
	if sc["terminate"] <= sc["model_call:cheap"] {
		t.Fatal("a finished task must terminate")
	}
}

func TestProbabilitiesAreNormalised(t *testing.T) {
	p := Probabilities(map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}, 1.0)
	sum := 0.0
	for _, v := range p {
		sum += v
	}
	if sum < 0.9999 || sum > 1.0001 {
		t.Fatalf("probabilities must sum to 1, got %f", sum)
	}
	if !(p["c"] > p["b"] && p["b"] > p["a"]) {
		t.Fatalf("ordering must be preserved: %+v", p)
	}
}

func TestDecideNeverInventsACandidate(t *testing.T) {
	state := &veyrav1.CommonExecutionState{Step: 3}
	allowed := []*veyrav1.ActionCandidate{{Id: "model_call:cheap", Type: veyrav1.ActionType_MODEL_CALL, ExpectedSuccess: 0.5}}
	req := &veyrav1.DecisionRequest{State: state, Candidates: allowed}
	d := Decide(NewHeuristic(), req, allowed, map[string]string{})
	if d.ChosenId != "model_call:cheap" {
		t.Fatalf("unexpected choice %q", d.ChosenId)
	}
	if len(d.CandidateIds) != 1 || d.CandidateIds[0] != "model_call:cheap" {
		t.Fatalf("decision must only reference the allowed set, got %+v", d.CandidateIds)
	}
}