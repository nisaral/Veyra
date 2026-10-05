// Package policy applies hard, deterministic constraints to a candidate set.
//
// The separation is the whole point of the project:
//
//	DecisionModel  predicts  (probabilities over candidates)
//	Policy         decides  (what is legal, affordable, safe)
//	Executor       acts
//	Verifier       validates
//
// A decision backend can never widen the action space.
package policy

import (
	"fmt"
	"sort"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

// Context is the deterministic state the constraints read.
type Context struct {
	Step               int64
	Fraction           float64
	FractionSoftLimit  float64
	RetriesInARow      int
	MaxRetriesInARow   int
	CurrentHarness     string
	HasOtherHarness    bool
	RemainingUSD       float64
	GrantedPermissions []string
	MinSteps           int64
	DeclaredDone       bool
	DeclaredFailed     bool
}

// DefaultContext returns the v0.1 policy defaults.
func DefaultContext() Context {
	return Context{
		FractionSoftLimit: 0.75,
		MaxRetriesInARow:  2,
		MinSteps:          2,
	}
}

// Result is the filtered set plus why things were removed (for the audit trail).
type Result struct {
	Allowed []*veyrav1.ActionCandidate
	Dropped map[string]string
}

// Filter removes every candidate that violates a hard constraint.
func Filter(cands []*veyrav1.ActionCandidate, ctx Context) Result {
	granted := map[string]bool{}
	for _, p := range ctx.GrantedPermissions {
		granted[p] = true
	}

	res := Result{Dropped: map[string]string{}}
	for _, c := range cands {
		if reason := violate(c, ctx, granted); reason != "" {
			res.Dropped[c.Id] = reason
			continue
		}
		res.Allowed = append(res.Allowed, c)
	}
	sort.SliceStable(res.Allowed, func(i, j int) bool { return res.Allowed[i].Id < res.Allowed[j].Id })
	return res
}

func violate(c *veyrav1.ActionCandidate, ctx Context, granted map[string]bool) string {
	for _, p := range c.RequiredPermissions {
		if !granted[p] {
			return fmt.Sprintf("missing permission %q", p)
		}
	}
	switch c.Type {
	case veyrav1.ActionType_SWITCH_HARNESS:
		if !ctx.HasOtherHarness {
			return "no alternative harness is available"
		}
		if ctx.Fraction >= ctx.FractionSoftLimit {
			return fmt.Sprintf("budget fraction %.2f >= soft limit %.2f", ctx.Fraction, ctx.FractionSoftLimit)
		}
	case veyrav1.ActionType_RETRY:
		if ctx.RetriesInARow >= ctx.MaxRetriesInARow {
			return fmt.Sprintf("retry streak %d >= %d", ctx.RetriesInARow, ctx.MaxRetriesInARow)
		}
	case veyrav1.ActionType_TERMINATE:
		if !ctx.DeclaredDone && !ctx.DeclaredFailed && ctx.Step < ctx.MinSteps {
			return fmt.Sprintf("cannot terminate before step %d", ctx.MinSteps)
		}
	case veyrav1.ActionType_MODEL_CALL:
		if ctx.RemainingUSD > 0 && c.EstCostUsd > ctx.RemainingUSD {
			return fmt.Sprintf("estimated cost $%.4f exceeds remaining $%.4f", c.EstCostUsd, ctx.RemainingUSD)
		}
	}
	return ""
}

// FixedHarnessOnly is the control condition: never switch, never re-plan.
// It keeps exactly one model tier plus termination, which is what a static
// harness (ReAct loop or a fixed LangGraph) actually does.
func FixedHarnessOnly(cands []*veyrav1.ActionCandidate) []*veyrav1.ActionCandidate {
	out := make([]*veyrav1.ActionCandidate, 0, 2)
	for _, c := range cands {
		switch c.Id {
		case "model_call:cheap", "terminate":
			out = append(out, c)
		}
	}
	return out
}