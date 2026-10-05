// Package kernelsrv exposes the control plane: submit, watch, inspect, cancel.
package kernelsrv

import (
	"context"
	"fmt"
	"io"
	"sync"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/engine"
	"github.com/nisaral/veyra/internal/eventlog"
	"github.com/nisaral/veyra/internal/runstore"
)

// HarnessPlane is what the server needs from the Python side.
type HarnessPlane interface {
	engine.HarnessClient
	List(ctx context.Context) ([]*veyrav1.HarnessInfo, error)
}

// DeciderFactory builds a decider for a requested backend name.
type DeciderFactory func(backend string) (engine.Decider, engine.Options)

// Server implements KernelService.
type Server struct {
	veyrav1.UnimplementedKernelServiceServer

	Store   *runstore.Store
	Plane   HarnessPlane
	Factory DeciderFactory

	mu   sync.Mutex
	stop map[string]context.CancelFunc
}

func New(store *runstore.Store, plane HarnessPlane, factory DeciderFactory) *Server {
	return &Server{Store: store, Plane: plane, Factory: factory, stop: map[string]context.CancelFunc{}}
}

func (s *Server) Submit(ctx context.Context, req *veyrav1.SubmitRequest) (*veyrav1.SubmitResponse, error) {
	task := req.GetTask()
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	run, err := s.Store.Create(task, req.GetRunId())
	if err != nil {
		return nil, err
	}

	opts := engine.Options{
		PolicyBackend: req.GetPolicyBackend(),
		StartHarness:  req.GetStartHarness(),
		OptionsJSON:   req.GetOptionsJson(),
		Tools:         []string{"read_file", "write_file", "list_dir", "grep", "run_command"},
		Verify:        true,
		MaxSteps:      60,
	}
	var dec engine.Decider
	if s.Factory != nil {
		dec, opts = s.Factory(req.GetPolicyBackend())
		if req.GetStartHarness() != "" {
			opts.StartHarness = req.GetStartHarness()
		}
		if req.GetOptionsJson() != "" {
			opts.OptionsJSON = req.GetOptionsJson()
		}
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.stop[run.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.stop, run.ID)
			s.mu.Unlock()
			cancel()
		}()
		_ = engine.Run(runCtx, run, s.Plane, dec, opts)
	}()

	return &veyrav1.SubmitResponse{RunId: run.ID}, nil
}

func (s *Server) Watch(req *veyrav1.WatchRequest, stream veyrav1.KernelService_WatchServer) error {
	run, ok := s.Store.Get(req.GetRunId())
	if !ok {
		return fmt.Errorf("unknown run %q", req.GetRunId())
	}
	// Replay the log first so watchers never miss events between submit and watch.
	hist, err := eventlog.Read(run.EventPath())
	if err != nil && len(hist) == 0 {
		return err
	}
	for _, ev := range hist {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	ch, unsub := run.Subscribe()
	defer unsub()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}

func (s *Server) GetRun(_ context.Context, req *veyrav1.GetRunRequest) (*veyrav1.RunRecord, error) {
	run, ok := s.Store.Get(req.GetRunId())
	if !ok {
		return nil, fmt.Errorf("unknown run %q", req.GetRunId())
	}
	return run.Record()
}

func (s *Server) Cancel(_ context.Context, req *veyrav1.CancelRequest) (*veyrav1.CancelResponse, error) {
	s.mu.Lock()
	cancel, ok := s.stop[req.GetRunId()]
	s.mu.Unlock()
	if !ok {
		return &veyrav1.CancelResponse{Ok: false}, nil
	}
	cancel()
	return &veyrav1.CancelResponse{Ok: true}, nil
}

func (s *Server) ListHarnesses(ctx context.Context, _ *veyrav1.ListHarnessesRequest) (*veyrav1.ListHarnessesResponse, error) {
	if s.Plane == nil {
		return &veyrav1.ListHarnessesResponse{}, nil
	}
	hs, err := s.Plane.List(ctx)
	if err != nil {
		return nil, err
	}
	return &veyrav1.ListHarnessesResponse{Harnesses: hs}, nil
}

var _ io.Reader