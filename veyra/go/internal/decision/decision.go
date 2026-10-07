// Package decision holds the kernel-side decision backends.
//
// Every backend obeys the same contract:
//
//	state + legal candidates -> probabilities over candidate ids
//
// A backend never executes anything and never sees the action space widen.
package decision

import (
	"fmt"
	"math"
	"sort"
	"strings"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

// Backend is a decision model.
type Backend interface {
	Name() string
	Score(state *veyrav1.CommonExecutionState, allowed []*veyrav1.ActionCandidate) map[string]float64
}

// Heuristic is the zero-dependency default. It is intentionally readable and
// deliberately unlearned: it exists so the kernel, the contract tests and the
// control condition work with no Python and no model at all.
type Heuristic struct {
	Lambda float64 // cost aversion
	Mu     float64 // risk aversion
}

func NewHeuristic() Heuristic { return Heuristic{Lambda: 40, Mu: 0.6} }

func (h Heuristic) Name() string { return "heuristic" }

func (h Heuristic) Score(state *veyrav1.CommonExecutionState, allowed []*veyrav1.ActionCandidate) map[string]float64 {
	scores := map[string]float64{}
	if state == nil {
		state = &veyrav1.CommonExecutionState{}
	}
	for _, c := range allowed {
		s := c.ExpectedSuccess - h.Lambda*c.EstCostUsd - h.Mu*c.Risk
		switch c.Type {
		case veyrav1.ActionType_TERMINATE:
			// Terminating is the right answer when the task is done, the wrong
			// answer while there is budget left to recover. A fixed harness
			// gives up here; the adaptive policy should not.
			switch {
			case state.Done:
				s = 10
			case state.Failed:
				s = -0.15
			default:
				s = -0.5
			}
		case veyrav1.ActionType_RETRY:
			if state.Failed {
				s += 0.20
			}
		case veyrav1.ActionType_VERIFY:
			// Verifying before any work has happened is waste; verify once the
			// run has a plausible result, or once the harness claims success.
			if state.Step >= 3 {
				s += 0.10
			} else {
				s -= 0.45
			}
			if state.Done {
				s += 0.30
			}
		case veyrav1.ActionType_SWITCH_HARNESS:
			// A cheap harness can outscore a model call on raw expected
			// success. Switching is allowed only after the current harness
			// has failed; otherwise the controller leaves a working loop.
			if state.Failed {
				s += 0.18
			} else {
				s -= 0.45
			}
		case veyrav1.ActionType_TOOL_CALL:
			if len(state.ToolResults) == 0 {
				s += 0.05
			}
		case veyrav1.ActionType_MODEL_CALL:
			if state.Failed {
				s -= 0.10
			}
		}
		scores[c.Id] = s
	}
	return scores
}

// Probabilities converts scores to a probability distribution (softmax).
func Probabilities(scores map[string]float64, temperature float64) map[string]float64 {
	if temperature <= 0 {
		temperature = 1.0
	}
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	mx := math.Inf(-1)
	for _, id := range ids {
		mx = math.Max(mx, scores[id]/temperature)
	}
	sum := 0.0
	exps := make(map[string]float64, len(ids))
	for _, id := range ids {
		e := math.Exp(scores[id]/temperature - mx)
		exps[id] = e
		sum += e
	}
	out := make(map[string]float64, len(ids))
	for _, id := range ids {
		out[id] = exps[id] / sum
	}
	return out
}

// Decide runs a backend and resolves its preference into a concrete Decision.
func Decide(b Backend, req *veyrav1.DecisionRequest, allowed []*veyrav1.ActionCandidate, dropped map[string]string) *veyrav1.Decision {
	scores := b.Score(req.GetState(), allowed)
	probs := Probabilities(scores, 1.0)

	ids := make([]string, 0, len(allowed))
	for _, c := range allowed {
		ids = append(ids, c.Id)
	}
	sort.Strings(ids)

	chosen := ""
	best := math.Inf(-1)
	for _, id := range ids {
		if probs[id] > best {
			best = probs[id]
			chosen = id
		}
	}
	// Degenerate fallback: no probabilities at all -> highest expected success.
	if chosen == "" {
		for _, c := range allowed {
			if float64(c.ExpectedSuccess) > best {
				best = float64(c.ExpectedSuccess)
				chosen = c.Id
			}
		}
	}

	ordered := make([]string, 0, len(ids))
	ordered = append(ordered, ids...)
	orderedProbs := make([]float64, 0, len(ids))
	for _, id := range ordered {
		orderedProbs = append(orderedProbs, probs[id])
	}

	rationale := []string{}
	for _, id := range ids {
		if d, ok := dropped[id]; ok {
			rationale = append(rationale, fmt.Sprintf("%s: dropped (%s)", id, d))
		}
	}
	rationale = append(rationale, fmt.Sprintf("chose %s (p=%.3f) via %s", chosen, best, b.Name()))

	return &veyrav1.Decision{
		CandidateIds:  ordered,
		Probabilities: orderedProbs,
		ChosenId:      chosen,
		Confidence:    best,
		PolicyId:      "default-v0.1",
		Backend:       b.Name(),
		Rationale:     strings.Join(rationale, "; "),
	}
}

// Fixed is the static-harness control: always take the cheap model step.
// It cannot switch, verify or retry, which is exactly the fixed-workflow arm.
type Fixed struct{}

func (Fixed) Name() string { return "fixed" }

func (Fixed) Score(state *veyrav1.CommonExecutionState, allowed []*veyrav1.ActionCandidate) map[string]float64 {
	scores := map[string]float64{}
	for _, c := range allowed {
		switch {
		case c.Id == "terminate" && state.GetFailed():
			// A static harness gives up when the environment breaks.
			scores[c.Id] = 1.0
		case c.Id == "model_call:cheap":
			if state.GetFailed() {
				scores[c.Id] = 0.2
			} else {
				scores[c.Id] = 1.0
			}
		case c.Id == "terminate":
			scores[c.Id] = 0.5
		default:
			scores[c.Id] = -1.0
		}
	}
	return scores
}