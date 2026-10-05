// Package pyclient speaks the Python harness/decision plane over gRPC.
//
// One Python process serves both DecisionService and HarnessService on a single
// port, so the kernel only ever manages one connection.
package pyclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

// Client implements engine.HarnessClient and engine.Decider.
type Client struct {
	conn *grpc.ClientConn
	hs   veyrav1.HarnessServiceClient
	ds   veyrav1.DecisionServiceClient
	name string
}

// Dial connects to a Python sidecar. It fails fast with a useful message
// because "the model plane is not running" is the most common operator error.
func Dial(ctx context.Context, addr, backendName string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	c := &Client{
		conn: conn,
		hs:   veyrav1.NewHarnessServiceClient(conn),
		ds:   veyrav1.NewDecisionServiceClient(conn),
		name: backendName,
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := c.hs.ListHarnesses(probe, &veyrav1.ListHarnessesRequest{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("python sidecar at %s is not answering ListHarnesses: %w", addr, err)
	}
	return c, nil
}

func (c *Client) Close() error { return c.conn.Close() }

// --- engine.HarnessClient -------------------------------------------------

func (c *Client) Start(ctx context.Context, runID, harnessID string, task *veyrav1.TaskSpec, state *veyrav1.CommonExecutionState, optionsJSON string) (*veyrav1.CommonExecutionState, error) {
	resp, err := c.hs.Start(ctx, &veyrav1.StartRequest{
		RunId:       runID,
		HarnessId:   harnessID,
		Task:        task,
		State:       state,
		OptionsJson: optionsJSON,
	})
	if err != nil {
		return nil, err
	}
	return resp.State, nil
}

func (c *Client) Step(ctx context.Context, runID, harnessID string, state *veyrav1.CommonExecutionState, dec *veyrav1.Decision) (*veyrav1.CommonExecutionState, error) {
	resp, err := c.hs.Step(ctx, &veyrav1.StepRequest{RunId: runID, State: state, Decision: dec})
	if err != nil {
		return nil, err
	}
	_ = harnessID
	return resp.State, nil
}

func (c *Client) Control(ctx context.Context, runID, harnessID string, op veyrav1.ControlOp) error {
	_, err := c.hs.Control(ctx, &veyrav1.ControlRequest{RunId: runID, HarnessId: harnessID, Op: op})
	return err
}

func (c *Client) List(ctx context.Context) ([]*veyrav1.HarnessInfo, error) {
	resp, err := c.hs.ListHarnesses(ctx, &veyrav1.ListHarnessesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Harnesses, nil
}

// --- engine.Decider -------------------------------------------------------

func (c *Client) Name() string { return c.name }

func (c *Client) Decide(ctx context.Context, req *veyrav1.DecisionRequest, _ []*veyrav1.ActionCandidate, _ map[string]string) (*veyrav1.Decision, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.ds.Decide(ctx, req)
}