// Package planner turns a task state into the set of legal next actions.
//
// This is the candidate generator, not the chooser. It proposes; the policy
// engine then removes anything unsafe or unaffordable, and only then does a
// decision backend get to express a preference.
//
// v0.1 deliberately proposes few candidates. The harness owns the inner ReAct
// loop and therefore owns tool selection; the kernel routes at checkpoints:
// which model tier, whether to retry, whether to move harness, when to stop.
// Verification is not a candidate here because the kernel already routes it
// deterministically the moment a harness claims success.
package planner

import (
	"fmt"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

// HarnessRef is the planner's view of an available harness.
type HarnessRef struct {
	ID                 string
	Capabilities       []string
	EstCostUSDPerStep  float64
	EstLatencyMS       int64
	SupportsCheckpoint bool
}

// Input is everything the planner may look at.
type Input struct {
	State          *veyrav1.CommonExecutionState
	Harnesses      []HarnessRef
	CurrentHarness string
	RetriesInARow  int
	MinSteps       int64
	// Tools is accepted for forward compatibility. v0.1 does not propose
	// TOOL_CALL candidates because only the harness knows the tool arguments.
	Tools          []string
	// TriedHarnesses are excluded from switching: without this the policy can
	// oscillate between two harnesses forever.
	TriedHarnesses []string
}

// ModelTier is one model size behind the same harness.
type ModelTier struct {
	Name         string
	EstCostUSD   float64
	EstLatencyMS int64
	Expected     float64
}

var (
	CheapTier  = ModelTier{Name: "cheap", EstCostUSD: 0.0010, EstLatencyMS: 1500, Expected: 0.55}
	StrongTier = ModelTier{Name: "strong", EstCostUSD: 0.0100, EstLatencyMS: 6000, Expected: 0.78}
)

// Candidates proposes the next-action set for this state.
func Candidates(in Input) []*veyrav1.ActionCandidate {
	state := in.State
	if state == nil {
		state = &veyrav1.CommonExecutionState{}
	}
	out := []*veyrav1.ActionCandidate{}

	if !state.GetDone() && !state.GetFailed() {
		out = append(out, modelCandidate(CheapTier, in.CurrentHarness))
		out = append(out, modelCandidate(StrongTier, in.CurrentHarness))
		if chars := contextChars(state); chars >= compactMinChars {
			out = append(out, &veyrav1.ActionCandidate{
				Id:              "compact_context",
				Type:            veyrav1.ActionType_TOOL_CALL,
				Provider:        in.CurrentHarness,
				Capabilities:    []string{"compact"},
				EstCostUsd:      0.0002,
				EstLatencyMs:    200,
				ExpectedSuccess: 0.70,
				Risk:            0.05,
				PayloadJson:     fmt.Sprintf(`{"chars":%d,"keep_first":1,"keep_last":4}`, chars),
				Rationale:       "drop the middle of the message history and keep a short summary",
			})
		}
	}

	if state.GetFailed() || in.RetriesInARow > 0 {
		out = append(out, &veyrav1.ActionCandidate{
			Id:              "retry",
			Type:            veyrav1.ActionType_RETRY,
			Provider:        in.CurrentHarness,
			Capabilities:    []string{"retry"},
			EstCostUsd:      0.0020,
			EstLatencyMs:    2000,
			ExpectedSuccess: 0.45,
			Risk:            0.30,
			Rationale:       "re-run the last failed action with a repaired approach",
		})
	}

	tried := map[string]bool{}
	for _, t := range in.TriedHarnesses {
		tried[t] = true
	}
	for _, h := range in.Harnesses {
		if h.ID == in.CurrentHarness || tried[h.ID] {
			continue
		}
		out = append(out, &veyrav1.ActionCandidate{
			Id:              "switch_harness:" + h.ID,
			Type:            veyrav1.ActionType_SWITCH_HARNESS,
			Provider:        h.ID,
			Capabilities:    h.Capabilities,
			EstCostUsd:      h.EstCostUSDPerStep,
			EstLatencyMs:    h.EstLatencyMS,
			ExpectedSuccess: 0.72,
			Risk:            0.25,
			PayloadJson:     fmt.Sprintf(`{"harness":%q,"checkpoint":%q}`, h.ID, state.GetCheckpointId()),
			Rationale:       "move this run to a different execution harness at a checkpoint",
		})
	}

	if state.GetDone() || state.GetFailed() || state.GetStep() >= in.MinSteps {
		out = append(out, &veyrav1.ActionCandidate{
			Id:              "terminate",
			Type:            veyrav1.ActionType_TERMINATE,
			Provider:        in.CurrentHarness,
			Capabilities:    []string{"terminate"},
			EstCostUsd:      0,
			EstLatencyMs:    0,
			ExpectedSuccess: 1.0,
			Risk:            0.0,
			Rationale:       "stop the run and report the final state",
		})
	}
	return out
}

// compactMinChars is the point at which a context condenser is worth a step.
// Below this, proposing compaction would steal the turn from a model call.
const compactMinChars = 6000

func contextChars(state *veyrav1.CommonExecutionState) int {
	n := 0
	for _, m := range state.GetMessages() {
		n += len(m.GetContent())
	}
	return n
}

func modelCandidate(t ModelTier, provider string) *veyrav1.ActionCandidate {
	return &veyrav1.ActionCandidate{
		Id:              "model_call:" + t.Name,
		Type:            veyrav1.ActionType_MODEL_CALL,
		Provider:        provider,
		Capabilities:    []string{"reason", "act"},
		EstCostUsd:      t.EstCostUSD,
		EstLatencyMs:    t.EstLatencyMS,
		ExpectedSuccess: t.Expected,
		Risk:            0.10,
		PayloadJson:     fmt.Sprintf(`{"tier":%q}`, t.Name),
		Rationale:       "one reasoning+acting step on the " + t.Name + " model tier",
	}
}

// Index maps candidate id -> candidate.
func Index(cands []*veyrav1.ActionCandidate) map[string]*veyrav1.ActionCandidate {
	m := make(map[string]*veyrav1.ActionCandidate, len(cands))
	for _, c := range cands {
		m[c.GetId()] = c
	}
	return m
}

