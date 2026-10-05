# Veyra

**An adaptive agent harness that chooses the next tool, model, verification, retry, or abstention under a budget and a permission check.**

The product objective, Harbor-first evaluation, and Gate 1 stop rule are in [`docs/OBJECTIVE.md`](docs/OBJECTIVE.md). `veyra targets` prints the checklist. No Harbor or Terminal-Bench number has been measured yet.

The v0.1 prototype below is the kernel that still answers a narrower plumbing question about switching coding harnesses:

> **Can a lightweight, state-dependent controller improve the cost of long-horizon agent
> execution by switching between execution harnesses without materially reducing task success?**

It is deliberately *not* a claim that adaptive routing is better than everything else. Prior
work already covers model routing, plugin harnesses, harness evolution and solve-time adaptation
(see [What this is not](#what-this-is-not)). Veyra asks a narrower, testable question and
publishes the answer either way.

---

## Result

Model and tools are held constant across every arm; only the *execution strategy* changes.
Measured on the bundled 30-task suite, split 15 dev / 15 test before any results were seen.

Full numbers, caveats and per-category tables: `out/dev/comparison.md` and `out/test/comparison.md`
(reproduce with `make bench-dev && make train && make bench-test`).

**Dev split** (used only to pick the fixed baseline):

| arm | success | mean $ / task | switches/run |
|---|---:|---:|---:|
| `fixed:native` | 80.0% | $0.0078 | 0.00 |
| `fixed:repair` | 100.0% | $0.0096 | 0.00 |
| `fixed:langgraph` | 100.0% | $0.0190 | 0.00 |
| `veyra:heuristic` | 100.0% | $0.0096 | 0.20 |
| `veyra:von` (surrogate) | 100.0% | $0.0096 | 0.20 |
| `veyra:bandit` (untrained) | 100.0% | $0.0096 | 0.20 |
| `oracle` (offline ceiling) | 100.0% | $0.0096 | 0.20 |

`fixed:repair` wins on dev, so it becomes the held-out baseline.

**Held-out test split** (5 arms only — no re-selection):

| arm | success | mean $ / task | cost vs baseline |
|---|---:|---:|---:|
| `fixed:repair` (dev-selected baseline) | 100.0% | $0.0096 | — |
| `veyra:heuristic` | 100.0% | $0.0096 | 0% |
| `veyra:von` (surrogate) | 100.0% | $0.0096 | 0% |
| `veyra:bandit` (trained on dev) | 100.0% | $0.0096 | 0% |
| `oracle` (offline ceiling) | 100.0% | $0.0096 | 0% |

**Outcome: NOT SUPPORTED** against `fixed:repair`. Adaptive arms match its success and its mean
cost. They do not beat it. Runtime verdict: `out/test/verdict.json`.

The v0.1 comparison, before `repair` existed, selected `fixed:langgraph` and reported adaptive
arms 37.9% cheaper. That result is kept in `docs/PREREGISTRATION.md`. It is not the current suite.

### What the result actually shows

- **`repair` is the harness that beats the previous fixed arms.** `fixed:native` scores 0% on
  `recovery`. `fixed:langgraph` scores 100% and charges graph overhead on every step.
  `fixed:repair` scores 100% at about half the graph harness's cost, because it loads the
  failing file in the harness and edits before it executes.
- **Switching into repair ties starting there.** The controller stays on the cheap loop until a
  failure, then moves to `repair`. Easy tasks stay cheap. Recovery tasks pay the failed step
  plus the repair. The average lands on the same $0.0096.
- **The offline oracle ceiling equals the heuristic.** Learning adds nothing measurable on this
  suite. That is a reported null result about the learned policy.

> **These numbers come from the scripted offline model.** They validate the kernel, the harness
> adapters, the state contract, the ledger and the decision backends — i.e. the *plumbing* — and
> they are not a claim about model behaviour. The headline claim needs a live model
> (`--model-mode ollama`). See [Caveats](#caveats).

---

## How it works

```
        ┌──────────────────────────────────────────────────────────────┐
        │  Go kernel  (task lifecycle · ledger · checkpoints · gRPC)  │
        └──────────────────────────────────────────────────────────────┘
                │                                          ▲
   DecisionRequest│                                          │Decision
                ▼                                          │
   ┌────────────────────┐   candidates   ┌────────────────────────────┐
   │ planner + policy   │───────────────▶│ DecisionModel              │
   │ (what is legal /   │                │  heuristic | von | bandit  │
   │  affordable?)      │◀───────────────│  | oracle (offline only)   │
   └────────────────────┘   score/choice └────────────────────────────┘
                │
                ▼
   ┌────────────────────────────────────────────────────────────────────┐
   │ HarnessService: native ReAct  |  LangGraph  (one node per action)  │
   └────────────────────────────────────────────────────────────────────┘
```

The design invariant is enforced in code and tested:

```
DecisionModel predicts  →  Policy constrains  →  Executor acts  →  Verifier validates
```

A decision backend **can never widen the action space**. It returns a preference over a
candidate set that the planner proposed and the policy already filtered; if it names anything
else, the kernel records `decision_rejected` and re-resolves deterministically.

### Switching is a checkpoint, not a hack

Veyra does not reach into a harness's private state. `CommonExecutionState` (proto) is the
portable contract — messages, tool results, observations, variables, artifacts, usage, budget,
checkpoint id. Each adapter maps it in and out:

```
native state ──▶ CommonExecutionState ──checkpoint──▶ CommonExecutionState ──▶ langgraph state
                            ▲                                    │
                            └────── Veyra decides SWITCH ────────┘
```

That is why `switch_harness:<id>` is a first-class action with a real cost, not a serialization
trick.

### The ledger is authoritative

Every action is charged to a ledger before dispatch: model cost, wall time, tokens, **and a
per-step harness overhead**. Harness choice is therefore an economic decision, and
`cost_usd` in the tables is charged cost, not self-reported model spend.

---

## Quickstart

Prerequisites: Go 1.24+, Python 3.10+, `protoc` (only to regenerate the wire format).

```bash
pip install -e "./python[dev]"
cd go && go build -o ../dist/veyra.exe ./cmd/veyra && cd ..
veyra doctor
veyra compare --spawn --split dev --out out/dev
veyra view --runs out/dev
veyra train --runs-dir out/dev/runs --out policies/bandit.json
veyra compare --spawn --split test --out out/test --select-from out/dev --bandit-state policies/bandit.json
```

`make bench-dev`, `make train`, and `make bench-test` are the same three steps when `make` exists.

One-off, no Makefile:

```bash
cd go && go build -o ../dist/veyra ./cmd/veyra && cd ..
python -m veyra.cli compare --spawn --split dev --out out/dev
```

On Windows, `make` may be unavailable; the individual commands above are equivalent. Any
`compare --spawn` refuses to start if something is already listening on the kernel or sidecar
port, because a stale kernel silently serving old code is the fastest way to publish wrong
numbers.

### Live-model mode

```bash
python -m veyra.cli compare --spawn --split dev --out out/live \
  --model-mode ollama --model-cheap qwen2.5-coder:3b --model-strong qwen2.5-coder:7b
```

`--model-mode openai` targets any OpenAI-compatible endpoint (vLLM, SGLang, hosted).

---

## Decision backends

| backend | what it is | honest label in the log |
|---|---|---|
| `heuristic` | readable hand-written rules; the unlearned baseline | `heuristic` |
| `von` | adapter for a Jev-style typed-probability model over HTTP (`POST /decide`) | `von` when an endpoint is configured, **`von-surrogate`** otherwise |
| `bandit` | LinUCB contextual bandit, fit offline from recorded traces | `bandit` when trained, **`bandit-untrained`** otherwise |
| `oracle` | **offline-only** ceiling: given the realized outcome, which harness would have been best at this checkpoint? | `oracle` |

Two rules keep these from being mistaken for more than they are:

- The `von` adapter with no endpoint is a small hand-tuned scorer, tagged `von-surrogate`. It is
  never presented as a Jev model. Point `--von-endpoint` at a real Von/Kev/NanoJev server to use
  one.
- An untrained `bandit` does not invent a policy; it defers to `heuristic` and is tagged
  `bandit-untrained`. Training reads the exact candidate set the kernel recorded in each decision
  event, so the policy is never fit on a guessed context.

The offline `oracle` arm exists to bound the value of switching: if an adaptive policy gets close
to the oracle, switching is being captured well; if it does not, that is informative too.

---

## The benchmark

30 deterministic tasks, 6 per category, split 15/15 dev/test before any results were seen.
Categories are chosen so that *execution strategy* can plausibly matter:

| category | why it is in the suite |
|---|---|
| `coding` | short, well-scoped; a plain loop is enough |
| `terminal` | tool-heavy shell/filesystem round trips |
| `recovery` | the first approach breaks; escalation is the point |
| `migration` | a rewrite against a changed interface |
| `long_horizon` | five or more dependent steps |
| — | each task carries a seed workspace, tools, budget, deterministic grader and optional fault injection |

Grader-based success is the ground truth; graders never call a model. `SWE-bench` is a
deliberate non-goal for v0.1 (its harness needs substantial machine resources); the task format is
small enough that an adapter is a later, separable piece of work.

---

## Repository layout

```
proto/veyra/v1/runtime.proto   wire format: kernel <-> decision <-> harness (source of truth)
go/                            kernel: lifecycle, ledger, checkpoints, event log, gRPC
  internal/{engine,planner,policy,budget,decision,runstore,eventlog,kernelsrv,pyclient,cli}
python/src/veyra/              sidecar: harness adapters, decision backends, benchmark, graders
  harness/{base,native,langgraph_harness,registry}.py
  decision/{base,heuristic,von,bandit,oracle}.py
  bench/{tasks,runner,report}.py
python/tests/                  pytest suite (tools, graders, contract, decisions, harnesses, report)
docs/PREREGISTRATION.md        the claim, the falsification bar, and the threats to validity
docs/ARCHITECTURE.md           why the kernel is small and where the boundaries are
```

The Go kernel is **infrastructure, not the contribution**. It owns task lifecycle, checkpointing,
budget, the event log and gRPC — deliberately nothing else. Python owns decision models, harness
adapters, the benchmark, graders and policy fitting. There is no Kubernetes, Postgres, ClickHouse
or NATS in v0.1; run logs are files.

---

## What this is not

Veyra is **not** "the first adaptive/self-evolving agent harness", and this README will not claim
that. Related work already occupies that space:

- harness routing, plugins, benchmark integration, RL and harness evolution (HarnessX-likes),
- solve-time adaptation and routing (adaptive auto-harness work),
- typed probabilistic decision models (Von / Kev / NanoJev and the Jev-style family),
- harness-vs-horizon studies, which motivate this project's focus on long-horizon and recovery
  tasks rather than short well-scoped ones.

Veyra's contribution is narrow and testable: **a portable execution-state contract, a small
kernel that can switch harnesses at a checkpoint, and a pre-registered evaluation of whether that
switching pays for itself under a fixed model and tool set.**

## Caveats

- `model_mode=offline` (the committed numbers) uses a deterministic scripted model. It proves
  plumbing and protocol; it cannot show model behaviour.
- `langgraph` falls back to an in-repo `MiniGraph` when the `langgraph` package cannot be
  imported; the arm says so (`engine:builtin-graph`). Install the `langgraph` extra to use the
  real runtime.
- `veyra:von` is a **surrogate** unless `--von-endpoint` is set. It must never be reported as a
  real Jev-style model.
- The bandit is fit offline on logged decisions (offline policy fitting), which is not causal
  inference; the pre-registration says so explicitly.
- The suite is 30 tasks. Confidence intervals on 15-task splits are wide; treat the numbers as a
  signal, not a measurement.

## Status

v0.1 research prototype, working end-to-end (`go test`, `pytest`, and both benchmark splits pass).
Next steps are listed in `docs/ARCHITECTURE.md`.

## License

Apache-2.0 — see `LICENSE`.
