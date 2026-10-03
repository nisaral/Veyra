# Contributing

Thanks for looking. This is a research prototype; the most useful contributions are ones that
make the *claim* easier to test or harder to fool.

## Development setup

```bash
pip install -e "./python[dev,langgraph,plot]"
make build          # dist/veyra
make doctor
make test
```

## Before opening a PR

```bash
make lint           # go vet
make test-go        # go test ./internal/...
make test-py        # pytest
make proto-check    # regenerated wire format must match the committed one
```

CI runs the same commands.

## Ground rules

- **Never widen the action space from a backend.** Decision models score candidates; the planner
  proposes and the policy filters. If you add a backend, it must handle being handed a candidate
  set it did not choose.
- **Label honestly.** If something is a surrogate, a fallback or untrained, say so in the name the
  kernel records (`von-surrogate`, `bandit-untrained`, `engine:builtin-graph`). Benchmark tables
  are generated from kernel events, so a misleading label becomes a misleading result.
- **No silent fallbacks.** If a component degrades (graph engine, model endpoint, policy), emit an
  event or a capability string. Reviewers should be able to see it.
- **Keep the kernel small.** New kernel responsibility needs a sentence in
  `docs/ARCHITECTURE.md` explaining why it cannot live in Python.
- **Costs are charged, not asserted.** Benchmarks use the ledger, never a model's own estimate.
- **Changing the benchmark changes the claim.** Adding tasks or categories is welcome, but it
  re-opens pre-registration: update `docs/PREREGISTRATION.md` and re-run dev selection *before*
  the held-out split.

## Adding a task

Append a builder in `python/src/veyra/bench/tasks.py` following the existing categories: seed
workspace, deterministic grader spec, budget, and offline scripts for each harness. Tasks must be
deterministic and self-contained.

## Adding a harness

Implement `HarnessAdapter` in `python/src/veyra/harness/`, register it in `registry.py`, and make
`start()` able to adopt an existing `CommonExecutionState` (that is what makes it switchable).
Add a test that a step mutates the portable state and a test that resuming from a checkpoint works.

## Code style

Match the surrounding code. Python: type hints on public functions, no new runtime dependencies
without a good reason. Go: `gofmt`, no new kernel dependencies unless unavoidable.
