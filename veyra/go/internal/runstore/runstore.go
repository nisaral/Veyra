// Package runstore keeps the authoritative record of every run.
//
// Filesystem layout (one directory per run, replayable with no database):
//
//	runs/<run-id>/events.jsonl
//	runs/<run-id>/checkpoints/<n>.json
package runstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/budget"
	"github.com/nisaral/veyra/internal/eventlog"
)

// Run is a single task execution.
type Run struct {
	mu       sync.Mutex
	ID       string
	Task     *veyrav1.TaskSpec
	Budget   budget.Budget
	Status   veyrav1.RunStatus
	State    *veyrav1.CommonExecutionState
	Usage    budget.Usage
	Started  int64
	Ended    int64
	Err      string
	Events   []*veyrav1.RunEvent
	Ckpts    []string
	dir      string
	log      *eventlog.Writer
	subs     map[int]chan *veyrav1.RunEvent
	nextSub  int
}

func (r *Run) Dir() string { return r.dir }

// EventPath is the JSONL log for this run.
func (r *Run) EventPath() string { return filepath.Join(r.dir, "events.jsonl") }

// CheckpointDir holds portable execution-state snapshots.
func (r *Run) CheckpointDir() string { return filepath.Join(r.dir, "checkpoints") }

// Emit records an event, appends it to disk and fans it out to watchers.
func (r *Run) Emit(ev *veyrav1.RunEvent) {
	r.mu.Lock()
	ev.RunId = r.ID
	ev.Seq = int64(len(r.Events) + 1)
	if ev.TsMs == 0 {
		ev.TsMs = time.Now().UnixMilli()
	}
	ev.Usage = usageProto(r.Usage)
	r.Events = append(r.Events, ev)
	if r.log != nil {
		_ = r.log.Append(ev)
	}
	subs := make([]chan *veyrav1.RunEvent, 0, len(r.subs))
	for _, ch := range r.subs {
		subs = append(subs, ch)
	}
	r.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- ev:
		default: // never block the engine on a slow watcher
		}
	}
}

// Subscribe returns a channel of live events plus an unsubscribe func.
func (r *Run) Subscribe() (<-chan *veyrav1.RunEvent, func()) {
	ch := make(chan *veyrav1.RunEvent, 256)
	r.mu.Lock()
	id := r.nextSub
	r.nextSub++
	if r.subs == nil {
		r.subs = map[int]chan *veyrav1.RunEvent{}
	}
	r.subs[id] = ch
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, id)
		r.mu.Unlock()
		close(ch)
	}
}

// SetStatus updates lifecycle state and (on terminal states) closes the log.
func (r *Run) SetStatus(s veyrav1.RunStatus, errMsg string) {
	r.mu.Lock()
	r.Status = s
	if errMsg != "" {
		r.Err = errMsg
	}
	terminal := s == veyrav1.RunStatus_SUCCEEDED || s == veyrav1.RunStatus_FAILED ||
		s == veyrav1.RunStatus_CANCELLED || s == veyrav1.RunStatus_BUDGET_EXCEEDED
	if terminal {
		r.Ended = time.Now().UnixMilli()
	}
	log := r.log
	if terminal {
		r.log = nil
	}
	r.mu.Unlock()
	if terminal && log != nil {
		_ = log.Close()
	}
}

// StatusOf returns the current status.
func (r *Run) StatusOf() veyrav1.RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Status
}

// RecordCheckpoint stores a portable state snapshot and returns its id.
func (r *Run) RecordCheckpoint(state *veyrav1.CommonExecutionState) (string, error) {
	r.mu.Lock()
	id := fmt.Sprintf("cp-%04d", len(r.Ckpts)+1)
	r.mu.Unlock()
	path, err := eventlog.WriteCheckpoint(r.CheckpointDir(), id, state)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	r.Ckpts = append(r.Ckpts, path)
	r.mu.Unlock()
	return id, nil
}

// Record persists the run-level summary for `veyra inspect`.
func (r *Run) Record() (*veyrav1.RunRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := &veyrav1.RunRecord{
		RunId:      r.ID,
		Status:     r.Status,
		Task:       r.Task,
		State:      r.State,
		Usage:      usageProto(r.Usage),
		Budget:     budgetProto(r.Budget),
		StartedMs:  r.Started,
		EndedMs:    r.Ended,
		Error:      r.Err,
		Events:     append([]*veyrav1.RunEvent(nil), r.Events...),
		Checkpoints: append([]string(nil), r.Ckpts...),
	}
	return rec, nil
}

// Store owns all runs under a base directory.
type Store struct {
	mu    sync.Mutex
	runs  map[string]*Run
	base  string
	order []string
}

func NewStore(base string) *Store {
	return &Store{runs: map[string]*Run{}, base: base}
}

// Create allocates a new run and opens its event log.
func (s *Store) Create(task *veyrav1.TaskSpec, runID string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().UnixMilli())
	}
	if _, exists := s.runs[runID]; exists {
		return nil, fmt.Errorf("run %q already exists", runID)
	}
	dir := filepath.Join(s.base, runID)
	if err := os.MkdirAll(filepath.Join(dir, "checkpoints"), 0o755); err != nil {
		return nil, err
	}
	log, err := eventlog.NewWriter(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	r := &Run{
		ID:      runID,
		Task:    task,
		Status:  veyrav1.RunStatus_PENDING,
		Started: time.Now().UnixMilli(),
		dir:     dir,
		log:     log,
		subs:    map[int]chan *veyrav1.RunEvent{},
	}
	if task != nil && task.Budget != nil {
		r.Budget = budget.Budget{
			MaxActions: task.Budget.MaxActions,
			MaxUSD:     task.Budget.MaxUsd,
			MaxWallMS:  task.Budget.MaxWallMs,
			MaxTokens:  task.Budget.MaxTokens,
		}
	}
	s.runs[runID] = r
	s.order = append(s.order, runID)
	return r, nil
}

func (s *Store) Get(id string) (*Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	return r, ok
}

func (s *Store) List() []*Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Run, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.runs[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started < out[j].Started })
	return out
}

func usageProto(u budget.Usage) *veyrav1.BudgetUsage {
	return &veyrav1.BudgetUsage{Actions: u.Actions, Usd: u.USD, WallMs: u.WallMS, Tokens: u.Tokens}
}

func budgetProto(b budget.Budget) *veyrav1.Budget {
	return &veyrav1.Budget{MaxActions: b.MaxActions, MaxUsd: b.MaxUSD, MaxWallMs: b.MaxWallMS, MaxTokens: b.MaxTokens}
}