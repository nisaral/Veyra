// Package engine is the adaptive execution loop.
//
//	TASK STATE -> DECISION -> ACTION -> OBSERVATION -> UPDATED STATE -> ...
//
// The loop is deliberately the only place that mutates a run. Harnesses are
// stateless factories as far as the kernel is concerned: switching harnesses is
// `checkpoint -> interrupt old -> start new with the checkpoint`.
package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/budget"
	"github.com/nisaral/veyra/internal/decision"
	"github.com/nisaral/veyra/internal/eventlog"
	"github.com/nisaral/veyra/internal/planner"
	"github.com/nisaral/veyra/internal/policy"
	"github.com/nisaral/veyra/internal/runstore"
)

// HarnessClient is the kernel's view of the Python harness plane.
type HarnessClient interface {
	Start(ctx context.Context, runID, harnessID string, task *veyrav1.TaskSpec, state *veyrav1.CommonExecutionState, optionsJSON string) (*veyrav1.CommonExecutionState, error)
	Step(ctx context.Context, runID, harnessID string, state *veyrav1.CommonExecutionState, dec *veyrav1.Decision) (*veyrav1.CommonExecutionState, error)
	Control(ctx context.Context, runID, harnessID string, op veyrav1.ControlOp) error
	List(ctx context.Context) ([]*veyrav1.HarnessInfo, error)
}

// Decider resolves legal candidates into a single decision.
type Decider interface {
	Decide(ctx context.Context, req *veyrav1.DecisionRequest, allowed []*veyrav1.ActionCandidate, dropped map[string]string) (*veyrav1.Decision, error)
	Name() string
}

// Options configures one run.
type Options struct {
	PolicyBackend      string
	StartHarness       string
	Tools              []string
	OptionsJSON        string
	Verify             bool
	MaxSteps           int64
	Fallback           decision.Backend
	GrantedPermissions []string
}

// Run executes until the task terminates, the budget runs out or ctx is done.
func Run(ctx context.Context, run *runstore.Run, hc HarnessClient, dec Decider, opts Options) error {
	if opts.Fallback == nil {
		opts.Fallback = decision.NewHeuristic()
	}
	if opts.MaxSteps == 0 {
		opts.MaxSteps = 60
	}
	pc := policy.DefaultContext()
	pc.GrantedPermissions = opts.GrantedPermissions

	harnesses, err := hc.List(ctx)
	if err != nil {
		run.SetStatus(veyrav1.RunStatus_FAILED, err.Error())
		run.Emit(&veyrav1.RunEvent{Kind: "run_finished", Status: veyrav1.RunStatus_FAILED,
			JsonPayload: eventlog.MarshalJSON(map[string]any{"error": err.Error()})})
		return err
	}
	current := opts.StartHarness
	if current == "" && len(harnesses) > 0 {
		current = harnesses[0].GetId()
	}

	// Every action pays the harness's per-step overhead on top of whatever the
	// model reports. Native is cheap because it is just a loop; a graph runtime
	// carries real per-step cost. This is what makes harness selection an
	// economic decision instead of a free one.
	hcost := map[string]float64{}
	hlat := map[string]int64{}
	for _, h := range harnesses {
		hcost[h.GetId()] = h.GetEstCostUsdPerStep()
		hlat[h.GetId()] = h.GetEstLatencyMs()
	}
	chargeHarness := func(prev usageSnapshot, st *veyrav1.CommonExecutionState) {
		chargeDelta(run, prev, st, hcost[st.GetHarnessId()], hlat[st.GetHarnessId()])
	}

	run.SetStatus(veyrav1.RunStatus_RUNNING, "")
	run.Emit(&veyrav1.RunEvent{
		Kind:   "task_started",
		Status: veyrav1.RunStatus_RUNNING,
		JsonPayload: eventlog.MarshalJSON(map[string]any{
			"task": run.Task.GetId(), "instruction": run.Task.GetInstruction(),
			"harness": current, "policy": opts.PolicyBackend, "tools": opts.Tools,
		}),
	})

	state, err := hc.Start(ctx, run.ID, current, run.Task, nil, opts.OptionsJSON)
	if err != nil {
		return finish(run, veyrav1.RunStatus_FAILED, fmt.Sprintf("harness start failed: %v", err))
	}
	state.HarnessId = current
	markTried(state, current)
	run.State = state
	chargeHarness(usageSnapshot{}, state)

	verified := false
	retriesInARow := 0
	var lastFailure string

	for step := int64(0); step < opts.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return finish(run, veyrav1.RunStatus_CANCELLED, "cancelled")
		}
		if exceeded, why := run.Exceeded(); exceeded {
			return finish(run, veyrav1.RunStatus_BUDGET_EXCEEDED, why)
		}

		// Verification is a first-class action, not an afterthought: once the
		// harness claims success we still route a verify decision.
		if state.GetDone() && opts.Verify && !verified {
			vdec := &veyrav1.Decision{
				ChosenId: "verify", Backend: "kernel", PolicyId: "verification-gate",
				Rationale: "harness reported done; routing the verifier before accepting",
			}
			run.Emit(&veyrav1.RunEvent{Kind: "decision", Decision: vdec,
				JsonPayload: eventlog.MarshalJSON(map[string]any{"forced": "verify"})})
			next, err := hc.Step(ctx, run.ID, state.GetHarnessId(), state, vdec)
			if err != nil {
				return finish(run, veyrav1.RunStatus_FAILED, fmt.Sprintf("verify failed: %v", err))
			}
			state = next
			run.State = state
			verified = true
			run.Emit(&veyrav1.RunEvent{Kind: "verify", Status: statusFor(state), State: state,
				JsonPayload: eventlog.MarshalJSON(map[string]any{"passed": !state.GetFailed()})})
			continue
		}

		if state.GetDone() {
			return finish(run, veyrav1.RunStatus_SUCCEEDED, "")
		}

		// -- candidate generation + hard constraints -----------------------
		cands := planner.Candidates(planner.Input{
			State:          state,
			Harnesses:      toRefs(harnesses),
			CurrentHarness: state.GetHarnessId(),
			Tools:          opts.Tools,
			RetriesInARow:  retriesInARow,
			MinSteps:       pc.MinSteps,
			TriedHarnesses: triedHarnesses(state),
		})
		if opts.PolicyBackend == "fixed" || strings.HasPrefix(opts.PolicyBackend, "fixed:") {
			cands = policy.FixedHarnessOnly(cands)
		}
		pc.Step = state.GetStep()
		pc.Fraction = run.Fraction()
		pc.RetriesInARow = retriesInARow
		pc.DeclaredDone = state.GetDone()
		pc.DeclaredFailed = state.GetFailed()
		pc.CurrentHarness = state.GetHarnessId()
		pc.HasOtherHarness = len(harnesses) > 1
		pc.RemainingUSD = run.RemainingUSD()
		filtered := policy.Filter(cands, pc)
		if len(filtered.Allowed) == 0 {
			return finish(run, veyrav1.RunStatus_FAILED, "policy removed every candidate (deadlock)")
		}

		req := &veyrav1.DecisionRequest{
			State:      state,
			Candidates: filtered.Allowed,
			Question:   "next_action",
			Budget:     run.BudgetProto(),
			Usage:      run.UsageProto(),
			Backend:    opts.PolicyBackend,
		}

		var d *veyrav1.Decision
		if dec != nil {
			d, err = dec.Decide(ctx, req, filtered.Allowed, filtered.Dropped)
			if err != nil {
				run.Emit(&veyrav1.RunEvent{Kind: "decision_fallback",
					JsonPayload: eventlog.MarshalJSON(map[string]any{"decider": dec.Name(), "error": err.Error()})})
				d = nil
			}
		}
		if d == nil {
			d = decision.Decide(opts.Fallback, req, filtered.Allowed, filtered.Dropped)
		}
		byID := planner.Index(filtered.Allowed)
		chosen, ok := byID[d.GetChosenId()]
		if !ok {
			// A backend must not invent candidates. Record the rejection, then
			// re-resolve deterministically against the allowed set.
			run.Emit(&veyrav1.RunEvent{Kind: "decision_rejected", State: state,
				JsonPayload: eventlog.MarshalJSON(map[string]any{
					"backend": dec.Name(), "returned": d.GetChosenId(), "reason": "candidate not in allowed set",
				})})
			d = decision.Decide(opts.Fallback, req, filtered.Allowed, filtered.Dropped)
			chosen = byID[d.GetChosenId()]
			if chosen == nil {
				return finish(run, veyrav1.RunStatus_FAILED, "decision backend returned an unknown candidate")
			}
		}
		run.Emit(&veyrav1.RunEvent{Kind: "decision", Decision: d, State: state, Action: chosen,
			JsonPayload: eventlog.MarshalJSON(map[string]any{
				"dropped": filtered.Dropped, "allowed": len(filtered.Allowed),
				"candidates": candidateMeta(filtered.Allowed),
			})})

		// -- dispatch -------------------------------------------------------
		switch chosen.GetType() {
		case veyrav1.ActionType_TERMINATE:
			st := veyrav1.RunStatus_SUCCEEDED
			if state.GetFailed() {
				st = veyrav1.RunStatus_FAILED
			}
			return finish(run, st, lastFailure)

		case veyrav1.ActionType_SWITCH_HARNESS:
			target := chosen.GetProvider()
			cpID, err := run.RecordCheckpoint(state)
			if err != nil {
				return finish(run, veyrav1.RunStatus_FAILED, fmt.Sprintf("checkpoint failed: %v", err))
			}
			state.CheckpointId = cpID
			_ = hc.Control(ctx, run.ID, state.GetHarnessId(), veyrav1.ControlOp_INTERRUPT)
			run.Emit(&veyrav1.RunEvent{Kind: "checkpoint", State: state,
				JsonPayload: eventlog.MarshalJSON(map[string]any{
					"checkpoint": cpID, "from": state.GetHarnessId(), "to": target,
				})})

			newState, err := hc.Start(ctx, run.ID, target, run.Task, state, opts.OptionsJSON)
			if err != nil {
				return finish(run, veyrav1.RunStatus_FAILED, fmt.Sprintf("switch to %s failed: %v", target, err))
			}
			prev := state.GetHarnessId()
			state = newState
			state.HarnessId = target
			// The failure belongs to the harness we just left. Clearing it
			// gives the new harness one step before another switch is scored
			// as a recovery. Without this, a second harness is selected
			// before the first replacement acts.
			state.Failed = false
			state.FailureKind = ""
			markTried(state, target)
			run.State = state
			chargeHarness(usageSnapshot{}, state)
			run.Emit(&veyrav1.RunEvent{Kind: "harness_switch", Status: veyrav1.RunStatus_SWITCHED, State: state,
				JsonPayload: eventlog.MarshalJSON(map[string]any{"from": prev, "to": target, "checkpoint": cpID})})
			continue

		case veyrav1.ActionType_RETRY:
			retriesInARow++
		default:
			retriesInARow = 0
		}

		prev := usageOf(state)
		next, err := hc.Step(ctx, run.ID, state.GetHarnessId(), state, d)
		if err != nil {
			lastFailure = err.Error()
			run.Emit(&veyrav1.RunEvent{Kind: "failure", Action: chosen, State: state,
				JsonPayload: eventlog.MarshalJSON(map[string]any{"error": err.Error()})})
			if retriesInARow >= pc.MaxRetriesInARow {
				return finish(run, veyrav1.RunStatus_FAILED, err.Error())
			}
			retriesInARow++
			continue
		}
		state = next
		run.State = state
		if chosen.GetType() == veyrav1.ActionType_VERIFY {
			verified = true
		}
		chargeHarness(prev, state)

		run.Emit(&veyrav1.RunEvent{Kind: "action", Action: chosen, State: state,
			JsonPayload: eventlog.MarshalJSON(map[string]any{
				"type": chosen.GetType().String(), "provider": chosen.GetProvider(),
			})})
		run.Emit(&veyrav1.RunEvent{Kind: "observation", State: state,
			JsonPayload: eventlog.MarshalJSON(map[string]any{
				"step": state.GetStep(), "done": state.GetDone(), "failed": state.GetFailed(),
				"observations": tail(state.GetObservations(), 3), "artifacts": len(state.GetArtifacts()),
			})})

		if state.GetFailed() {
			lastFailure = state.GetFailureKind()
			if lastFailure == "" {
				lastFailure = "harness reported failure"
			}
			retriesInARow++
		}
	}
	return finish(run, veyrav1.RunStatus_FAILED, fmt.Sprintf("max steps (%d) exceeded", opts.MaxSteps))
}

func finish(run *runstore.Run, status veyrav1.RunStatus, errMsg string) error {
	run.SetStatus(status, errMsg)
	run.Emit(&veyrav1.RunEvent{Kind: "run_finished", Status: status, State: run.State,
		JsonPayload: eventlog.MarshalJSON(map[string]any{"error": errMsg, "status": status.String()})})
	if status == veyrav1.RunStatus_FAILED && errMsg != "" {
		return errors.New(errMsg)
	}
	return nil
}

const triedKey = "veyra.tried_harnesses"

// markTried records harness attempts inside the portable state so the record
// survives checkpointing and harness switches. Without it the policy can
// oscillate between two harnesses forever.
func markTried(state *veyrav1.CommonExecutionState, id string) {
	if state.GetVariables() == nil {
		state.Variables = map[string]string{}
	}
	for _, t := range strings.Split(state.Variables[triedKey], ",") {
		if t == id {
			return
		}
	}
	if cur := state.Variables[triedKey]; cur != "" {
		state.Variables[triedKey] = cur + "," + id
	} else {
		state.Variables[triedKey] = id
	}
}

func triedHarnesses(state *veyrav1.CommonExecutionState) []string {
	raw := state.GetVariables()[triedKey]
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

type usageSnapshot struct {
	USD    float64
	WallMS int64
	Tokens int64
}

func usageOf(state *veyrav1.CommonExecutionState) usageSnapshot {
	u := state.GetUsage()
	return usageSnapshot{USD: u.GetUsd(), WallMS: u.GetWallMs(), Tokens: u.GetTokens()}
}

// chargeDelta bills the run for exactly what the harness reported, never for
// what the kernel assumed it would cost.
func chargeDelta(run *runstore.Run, prev usageSnapshot, state *veyrav1.CommonExecutionState, overheadUSD float64, overheadMS int64) {
	cur := usageOf(state)
	d := budget.Usage{Actions: 1, USD: overheadUSD, WallMS: overheadMS}
	if cur.USD > prev.USD {
		d.USD += cur.USD - prev.USD
	}
	if cur.WallMS > prev.WallMS {
		d.WallMS = cur.WallMS - prev.WallMS
	}
	if cur.Tokens > prev.Tokens {
		d.Tokens = cur.Tokens - prev.Tokens
	}
	run.Charge(d)
}

// candidateMeta records the exact legal candidate set in the decision event.
// Without it an offline learner would be fit on a guessed context instead of the
// features the policy actually saw at decision time.
func candidateMeta(cands []*veyrav1.ActionCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(cands))
	for _, c := range cands {
		out = append(out, map[string]any{
			"id":               c.GetId(),
			"type":             c.GetType().String(),
			"provider":         c.GetProvider(),
			"est_cost_usd":     c.GetEstCostUsd(),
			"expected_success": c.GetExpectedSuccess(),
			"risk":             c.GetRisk(),
			"latency_ms":       c.GetEstLatencyMs(),
		})
	}
	return out
}
func toRefs(hs []*veyrav1.HarnessInfo) []planner.HarnessRef {
	out := make([]planner.HarnessRef, 0, len(hs))
	for _, h := range hs {
		out = append(out, planner.HarnessRef{
			ID:                 h.GetId(),
			Capabilities:       h.GetCapabilities(),
			EstCostUSDPerStep:  h.GetEstCostUsdPerStep(),
			EstLatencyMS:       h.GetEstLatencyMs(),
			SupportsCheckpoint: h.GetSupportsCheckpoint(),
		})
	}
	return out
}

func tail(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return xs[len(xs)-n:]
}

func statusFor(s *veyrav1.CommonExecutionState) veyrav1.RunStatus {
	if s.GetFailed() {
		return veyrav1.RunStatus_FAILED
	}
	return veyrav1.RunStatus_RUNNING
}