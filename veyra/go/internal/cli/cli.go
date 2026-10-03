// Package cli implements the `veyra` command surface.
//
// `veyra run` executes in-process (no daemon needed for the demo); `veyra serve`
// exposes the same kernel over gRPC for the Python benchmark runner.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"net"
	"google.golang.org/grpc"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/engine"
	"github.com/nisaral/veyra/internal/eventlog"
	"github.com/nisaral/veyra/internal/kernelsrv"
	"github.com/nisaral/veyra/internal/pyclient"
	"github.com/nisaral/veyra/internal/runstore"
	"github.com/nisaral/veyra/internal/taskfile"
)

// Version is the kernel version string.
const Version = "0.1.0"

const defaultTools = "read_file,write_file,list_dir,grep,run_command"

func usage() {
	fmt.Print(`veyra - adaptive execution kernel

usage:
  veyra run "<instruction>" [--task FILE] [--harness native] [--policy heuristic]
  veyra serve [--addr 127.0.0.1:7788]
  veyra harness list [--python 127.0.0.1:7777]
  veyra inspect <run-id> [--runs-dir runs]
  veyra replay <run-id|events.jsonl>
  veyra doctor [--python 127.0.0.1:7777]
  veyra version

policy backends: heuristic | fixed | fixed:native | fixed:langgraph | von | bandit | oracle
`)
}

// Run dispatches a CLI invocation.
func Run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "run":
		return cmdRun(args[1:])
	case "serve":
		return cmdServe(args[1:])
	case "harness":
		return cmdHarness(args[1:])
	case "inspect":
		return cmdInspect(args[1:])
	case "replay":
		return cmdReplay(args[1:])
	case "doctor":
		return cmdDoctor(args[1:])
	case "version", "-v", "--version":
		fmt.Println("veyra " + Version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try `veyra help`)", args[0])
	}
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	taskPath := fs.String("task", "", "path to a task.yaml / task.json file")
	harness := fs.String("harness", "native", "harness to start in")
	policy := fs.String("policy", "heuristic", "decision backend")
	py := fs.String("python", "127.0.0.1:7777", "python sidecar address")
	runsDir := fs.String("runs-dir", "runs", "directory for run logs")
	maxActions := fs.Int64("max-actions", 30, "action budget")
	budgetUSD := fs.Float64("budget-usd", 0.50, "dollar budget")
	maxSteps := fs.Int64("max-steps", 40, "hard loop cap")
	toolsCSV := fs.String("tools", defaultTools, "tools exposed to the planner")
	verify := fs.Bool("verify", true, "route a verifier before accepting success")
	quiet := fs.Bool("quiet", false, "do not stream events")
	ws := fs.String("workspace", "", "workspace directory for the task")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var task *veyrav1.TaskSpec
	if *taskPath != "" {
		t, err := taskfile.Load(*taskPath)
		if err != nil {
			return err
		}
		task = t
	} else {
		instruction := strings.Join(fs.Args(), " ")
		if strings.TrimSpace(instruction) == "" {
			return fmt.Errorf("nothing to run: pass an instruction or --task FILE")
		}
		task = taskfile.Inline(instruction, *ws, nil)
	}
	if task.Budget == nil {
		task.Budget = &veyrav1.Budget{}
	}
	if *maxActions > 0 {
		task.Budget.MaxActions = *maxActions
	}
	if *budgetUSD > 0 {
		task.Budget.MaxUsd = *budgetUSD
	}
	if task.Workspace == "" && *ws != "" {
		task.Workspace = *ws
	}

	store := runstore.NewStore(*runsDir)
	run, err := store.Create(task, "")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := pyclient.Dial(ctx, *py, *policy)
	if err != nil {
		return fmt.Errorf("%w\nhint: start it with `veyra-py serve --addr %s`", err, *py)
	}
	defer client.Close()

	if !*quiet {
		ch, unsub := run.Subscribe()
		defer unsub()
		go func() {
			for ev := range ch {
				printEvent(ev)
			}
		}()
	}
	fmt.Printf("run %s  harness=%s policy=%s budget=$%.2f/%d actions\n", run.ID, *harness, *policy, task.Budget.MaxUsd, task.Budget.MaxActions)

	var dec engine.Decider
	if isRemotePolicy(*policy) {
		dec = client
	}
	opts := engine.Options{
		PolicyBackend: *policy,
		StartHarness:  *harness,
		Tools:         splitCSV(*toolsCSV),
		Verify:        *verify,
		MaxSteps:      *maxSteps,
	}
	err = engine.Run(ctx, run, client, dec, opts)
	rec, _ := run.Record()
	if rec != nil {
		fmt.Printf("status=%s actions=%d cost=$%.4f wall=%dms\n",
			rec.Status.String(), rec.Usage.GetActions(), rec.Usage.GetUsd(), rec.Usage.GetWallMs())
	}
	if err != nil {
		return err
	}
	return nil
}

func isRemotePolicy(p string) bool {
	switch p {
	case "von", "bandit", "oracle":
		return true
	}
	return false
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:7788", "kernel listen address")
	py := fs.String("python", "127.0.0.1:7777", "python sidecar address")
	runsDir := fs.String("runs-dir", "runs", "directory for run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := pyclient.Dial(ctx, *py, "remote")
	if err != nil {
		return fmt.Errorf("%w\nhint: start it with `veyra-py serve --addr %s`", err, *py)
	}
	defer client.Close()

	store := runstore.NewStore(*runsDir)
	srv := kernelsrv.New(store, client, func(backend string) (engine.Decider, engine.Options) {
		opts := engine.Options{PolicyBackend: backend, Tools: splitCSV(defaultTools), Verify: true, MaxSteps: 60}
		if isRemotePolicy(backend) {
			return client, opts
		}
		return nil, opts
	})

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	g := grpc.NewServer()
	veyrav1.RegisterKernelServiceServer(g, srv)
	fmt.Printf("kernel listening on %s (python=%s, runs=%s)\n", *addr, *py, *runsDir)
	errCh := make(chan error, 1)
	go func() { errCh <- g.Serve(lis) }()
	select {
	case <-ctx.Done():
		g.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

func cmdHarness(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return fmt.Errorf("usage: veyra harness list [--python ADDR]")
	}
	fs := flag.NewFlagSet("harness list", flag.ContinueOnError)
	py := fs.String("python", "127.0.0.1:7777", "python sidecar address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := pyclient.Dial(ctx, *py, "list")
	if err != nil {
		return err
	}
	defer client.Close()
	hs, err := client.List(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%-12s %-10s %-8s %-8s %s\n", "ID", "KIND", "COST/STEP", "LATENCY", "CAPABILITIES")
	for _, h := range hs {
		fmt.Printf("%-12s %-10s %-8.4f %-8d %s\n", h.Id, h.Kind, h.EstCostUsdPerStep, h.EstLatencyMs, strings.Join(h.Capabilities, ","))
	}
	return nil
}

func cmdInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	runsDir := fs.String("runs-dir", "runs", "directory for run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: veyra inspect <run-id>")
	}
	id := fs.Arg(0)
	events, err := eventlog.Read(filepath.Join(*runsDir, id, "events.jsonl"))
	if err != nil {
		return err
	}
	printSummary(id, events)
	return nil
}

func cmdReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	runsDir := fs.String("runs-dir", "runs", "directory for run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: veyra replay <run-id|events.jsonl>")
	}
	target := fs.Arg(0)
	path := target
	if !strings.HasSuffix(target, ".jsonl") {
		path = filepath.Join(*runsDir, target, "events.jsonl")
	}
	events, err := eventlog.Read(path)
	if err != nil {
		return err
	}
	for _, ev := range events {
		printEvent(ev)
	}
	return nil
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	py := fs.String("python", "127.0.0.1:7777", "python sidecar address")
	runsDir := fs.String("runs-dir", "runs", "directory for run logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println("kernel:  veyra " + Version)
	wd, _ := os.Getwd()
	fmt.Println("cwd:     " + wd)
	fmt.Println("runs:    " + *runsDir)
	if err := os.MkdirAll(*runsDir, 0o755); err != nil {
		fmt.Println("runs:    NOT WRITABLE: " + err.Error())
	} else {
		fmt.Println("runs:    writable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := pyclient.Dial(ctx, *py, "doctor")
	if err != nil {
		fmt.Println("python:  UNREACHABLE at " + *py)
		fmt.Println("         " + err.Error())
		return nil
	}
	defer client.Close()
	hs, err := client.List(ctx)
	if err != nil {
		fmt.Println("python:  reachable but ListHarnesses failed: " + err.Error())
		return nil
	}
	fmt.Printf("python:  OK at %s (%d harnesses)\n", *py, len(hs))
	for _, h := range hs {
		fmt.Printf("         - %s (%s)\n", h.Id, h.Kind)
	}
	return nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func printEvent(ev *veyrav1.RunEvent) {
	ts := time.UnixMilli(ev.TsMs).Format("15:04:05.000")
	line := fmt.Sprintf("%s  %-16s", ts, ev.Kind)
	if ev.Action != nil {
		line += "  " + ev.Action.Id
	}
	if ev.Decision != nil {
		line += fmt.Sprintf("  -> %s (p=%.2f, %s)", ev.Decision.ChosenId, ev.Decision.Confidence, ev.Decision.Backend)
	}
	if ev.JsonPayload != "" && ev.JsonPayload != "{}" {
		line += "  " + compact(ev.JsonPayload)
	}
	fmt.Println(line)
}

func compact(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	out := string(b)
	if len(out) > 160 {
		out = out[:157] + "..."
	}
	return out
}

func printSummary(id string, events []*veyrav1.RunEvent) {
	var status veyrav1.RunStatus
	var usage *veyrav1.BudgetUsage
	switches, decisions, failures := 0, 0, 0
	for _, ev := range events {
		switch ev.Kind {
		case "harness_switch":
			switches++
		case "decision":
			decisions++
		case "failure":
			failures++
		case "run_finished":
			status = ev.Status
		}
		if ev.Usage != nil {
			usage = ev.Usage
		}
	}
	fmt.Printf("run      %s\n", id)
	fmt.Printf("status   %s\n", status.String())
	if usage != nil {
		fmt.Printf("usage    actions=%d cost=$%.4f wall=%dms tokens=%d\n",
			usage.Actions, usage.Usd, usage.WallMs, usage.Tokens)
	}
	fmt.Printf("events   %d (decisions=%d switches=%d failures=%d)\n", len(events), decisions, switches, failures)
}