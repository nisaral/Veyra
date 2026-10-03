package engine

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/runstore"
)

// fakePlane is a deterministic two-harness environment:
//
//	native    : always fails with an environment error
//	langgraph : makes progress, then solves the task
//
// It is the smallest possible stand-in for the real Python harness plane, which
// is what makes it a useful contract test for the kernel loop.
type fakePlane struct {
	steps    int
	switches int
	controls int
}

func (f *fakePlane) List(context.Context) ([]*veyrav1.HarnessInfo, error) {
	return []*veyrav1.HarnessInfo{
		{Id: "native", Kind: "react", EstCostUsdPerStep: 0.001, EstLatencyMs: 900, SupportsCheckpoint: true, Capabilities: []string{"reason", "act"}},
		{Id: "langgraph", Kind: "graph", EstCostUsdPerStep: 0.004, EstLatencyMs: 1400, SupportsCheckpoint: true, Capabilities: []string{"reason", "act"}},
	}, nil
}

func (f *fakePlane) Start(_ context.Context, _, harnessID string, task *veyrav1.TaskSpec, state *veyrav1.CommonExecutionState, _ string) (*veyrav1.CommonExecutionState, error) {
	if state == nil {
		return &veyrav1.CommonExecutionState{
			TaskId: task.GetId(), Instruction: task.GetInstruction(), HarnessId: harnessID,
			Usage: &veyrav1.BudgetUsage{}, Variables: map[string]string{},
		}, nil
	}
	s := clone(state)
	s.HarnessId = harnessID
	return s, nil
}

func (f *fakePlane) Step(_ context.Context, _, harnessID string, state *veyrav1.CommonExecutionState, dec *veyrav1.Decision) (*veyrav1.CommonExecutionState, error) {
	f.steps++
	s := clone(state)
	s.Step++
	s.Usage = &veyrav1.BudgetUsage{
		Actions: s.Usage.GetActions() + 1,
		Usd:     s.Usage.GetUsd() + 0.001,
		WallMs:  s.Usage.GetWallMs() + 20,
		Tokens:  s.Usage.GetTokens() + 120,
	}
	if dec.GetChosenId() == "verify" {
		s.Done = true
		s.Failed = false
		s.Observations = append(s.Observations, "verifier passed")
		return s, nil
	}
	switch harnessID {
	case "native":
		s.Failed = true
		s.FailureKind = "environment"
		s.Observations = append(s.Observations, "native: tool connection refused")
	case "langgraph":
		if s.Step < 3 {
			s.Failed = false
			s.Observations = append(s.Observations, "langgraph: recovered context, partial progress")
		} else {
			s.Done = true
			s.Failed = false
			s.Observations = append(s.Observations, "langgraph: task solved")
		}
	}
	return s, nil
}

func (f *fakePlane) Control(_ context.Context, _, _ string, op veyrav1.ControlOp) error {
	if op == veyrav1.ControlOp_INTERRUPT {
		f.switches++
	}
	f.controls++
	return nil
}

func clone(s *veyrav1.CommonExecutionState) *veyrav1.CommonExecutionState {
	return proto.Clone(s).(*veyrav1.CommonExecutionState)
}

func newRun(t *testing.T, maxActions int64, maxUSD float64) *runstore.Run {
	t.Helper()
	store := runstore.NewStore(t.TempDir())
	run, err := store.Create(&veyrav1.TaskSpec{
		Id:          "t-1",
		Instruction: "make the failing tool work",
		Budget:      &veyrav1.Budget{MaxActions: maxActions, MaxUsd: maxUSD},
	}, "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}

func kinds(run *runstore.Run) map[string]int {
	out := map[string]int{}
	rec, _ := run.Record()
	for _, ev := range rec.Events {
		out[ev.Kind]++
	}
	return out
}

// The headline behaviour: when the started harness keeps failing, the adaptive
// policy routes the run to another harness at a checkpoint instead of giving up.
func TestAdaptivePolicySwitchesHarnessAfterFailure(t *testing.T) {
	run := newRun(t, 30, 1.0)
	plane := &fakePlane{}
	err := Run(context.Background(), run, plane, nil, Options{
		PolicyBackend: "heuristic",
		StartHarness:  "native",
		Tools:         []string{"read_file"},
		Verify:        true,
		MaxSteps:      30,
	})
	if err != nil {
		t.Fatalf("run errored: %v", err)
	}
	rec, _ := run.Record()
	if rec.Status != veyrav1.RunStatus_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %s (error=%q)", rec.Status, rec.Error)
	}
	k := kinds(run)
	if k["harness_switch"] != 1 {
		t.Fatalf("expected exactly one harness switch, got %d (%v)", k["harness_switch"], k)
	}
	if plane.switches != 1 {
		t.Fatalf("expected the kernel to interrupt the failed harness once, got %d", plane.switches)
	}
	if len(rec.Checkpoints) == 0 {
		t.Fatal("a switch must leave a portable checkpoint behind")
	}
}

// The control condition: a fixed harness never escapes a broken environment.
func TestFixedPolicyDoesNotSwitch(t *testing.T) {
	run := newRun(t, 30, 1.0)
	plane := &fakePlane{}
	err := Run(context.Background(), run, plane, nil, Options{
		PolicyBackend: "fixed",
		StartHarness:  "native",
		Tools:         []string{"read_file"},
		Verify:        true,
		MaxSteps:      30,
	})
	if err == nil {
		t.Fatal("a fixed harness should not succeed in a broken environment")
	}
	rec, _ := run.Record()
	if rec.Status != veyrav1.RunStatus_FAILED {
		t.Fatalf("expected FAILED, got %s", rec.Status)
	}
	if k := kinds(run); k["harness_switch"] != 0 {
		t.Fatalf("fixed policy must never switch harnesses, saw %d", k["harness_switch"])
	}
}

// Budgets are hard: the run must stop the moment a dimension is spent.
func TestBudgetStopsTheRun(t *testing.T) {
	run := newRun(t, 2, 1.0)
	plane := &fakePlane{}
	_ = Run(context.Background(), run, plane, nil, Options{
		PolicyBackend: "heuristic",
		StartHarness:  "native",
		Tools:         nil,
		MaxSteps:      30,
	})
	rec, _ := run.Record()
	if rec.Status != veyrav1.RunStatus_BUDGET_EXCEEDED && rec.Status != veyrav1.RunStatus_FAILED {
		t.Fatalf("expected the run to stop on budget, got %s", rec.Status)
	}
	if rec.Usage.GetActions() > 4 {
		t.Fatalf("action budget must be enforced, used %d", rec.Usage.GetActions())
	}
}

// A backend that hallucinates a candidate id must not be able to execute it.
func TestDecisionCannotInventCandidates(t *testing.T) {
	run := newRun(t, 10, 1.0)
	plane := &fakePlane{}
	_ = Run(context.Background(), run, plane, rogueDecider{}, Options{
		PolicyBackend: "rogue",
		StartHarness:  "native",
		Tools:         nil,
		MaxSteps:      8,
	})
	rec, _ := run.Record()

	rejections := 0
	for _, ev := range rec.Events {
		if ev.Kind == "decision_rejected" {
			rejections++
		}
		if ev.Action != nil && ev.Action.Id == "rm -rf /" {
			t.Fatal("kernel executed a candidate the policy never allowed")
		}
		if ev.Decision != nil && ev.Decision.ChosenId == "rm -rf /" {
			t.Fatal("an illegal decision was recorded as the chosen action")
		}
	}
	if rejections == 0 {
		t.Fatal("expected the kernel to record that the rogue backend was overruled")
	}
}

type rogueDecider struct{}

func (rogueDecider) Name() string { return "rogue" }

func (rogueDecider) Decide(_ context.Context, _ *veyrav1.DecisionRequest, _ []*veyrav1.ActionCandidate, _ map[string]string) (*veyrav1.Decision, error) {
	return &veyrav1.Decision{ChosenId: "rm -rf /", Confidence: 1.0, Backend: "rogue"}, nil
}

// An offline learner can only be fit if the log records what the policy actually
// saw. This is the contract the Python bandit trainer depends on.
func TestDecisionEventsRecordCandidateMetadata(t *testing.T) {
	run := newRun(t, 30, 2.0)
	plane := &fakePlane{}
	_ = Run(context.Background(), run, plane, nil, Options{
		PolicyBackend: "heuristic",
		StartHarness:  "native",
		Tools:         []string{"read_file"},
		Verify:        true,
		MaxSteps:      30,
	})
	rec, _ := run.Record()

	found := false
	for _, ev := range rec.Events {
		if ev.Kind != "decision" || ev.JsonPayload == "" {
			continue
		}
		var payload struct {
			Candidates []struct {
				ID              string  `json:"id"`
				Type            string  `json:"type"`
				Provider        string  `json:"provider"`
				EstCostUSD      float64 `json:"est_cost_usd"`
				ExpectedSuccess float64 `json:"expected_success"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(ev.JsonPayload), &payload); err != nil {
			t.Fatalf("decision payload is not valid json: %v", err)
		}
		if len(payload.Candidates) == 0 {
			continue
		}
		found = true
		hasChosen := false
		for _, c := range payload.Candidates {
			if c.ID == ev.Decision.GetChosenId() {
				hasChosen = true
			}
			if c.Type == "" {
				t.Fatalf("candidate %q was recorded without a type", c.ID)
			}
		}
		if !hasChosen {
			t.Fatalf("chosen candidate %q is missing from the recorded set", ev.Decision.GetChosenId())
		}
		first := payload.Candidates[0]
		if first.Type != "TERMINATE" && first.EstCostUSD == 0 {
			t.Fatal("candidate economics were not recorded; a learner would be fit on a guessed context")
		}
	}
	if !found {
		t.Fatal("no decision event carried candidate metadata")
	}
}

// Harness choice must be an economic decision, so the per-step overhead has to
// land in the ledger on top of whatever the harness reports for the model.
func TestHarnessOverheadIsChargedOnTopOfModelCost(t *testing.T) {
	run := newRun(t, 30, 2.0)
	plane := &fakePlane{}
	_ = Run(context.Background(), run, plane, nil, Options{
		PolicyBackend: "fixed",
		StartHarness:  "native",
		Tools:         []string{"read_file"},
		Verify:        true,
		MaxSteps:      30,
	})
	rec, _ := run.Record()
	reported := rec.State.GetUsage().GetUsd()
	if rec.Usage.GetUsd() <= reported {
		t.Fatalf("harness overhead was not charged: ledger $%.4f <= reported state $%.4f",
			rec.Usage.GetUsd(), reported)
	}
}
