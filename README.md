# Veyra

A switch controller, handoff compiler, and measurement kit for existing agent harnesses.

Veyra does not replace Claude Code, Codex, OpenHands, Terminus-2, or Omnigent. It decides whether to stay, switch, retry, verify, ask, or stop; what state the next harness may see; and whether that choice paid for itself.

The policy is the only component that may act. A typed decision model scores candidate items from the trace. It does not write the next message and does not call tools. If the controller errors, times out, or is uncertain, the current harness continues.

```bash
cd veyra
pip install -e "./python[dev]"
cd go && go build -o ../dist/veyra.exe ./cmd/veyra && cd ..
veyra doctor
veyra compare --spawn --split dev --out out/dev
veyra dashboard --runs out/dev
veyra headroom --trials path/to/trials.csv
```

## Status

No Harbor or Terminal-Bench score has been published. Internal 30-task and Kev tables are synthetic or scripted. MCPAgentBench is a smoke test.

## Design

| Component | Role |
|---|---|
| Go kernel | Execution loop, budget, fail-open fallback, append-only event log |
| Python sidecar | Harness adapters, Kev client, analysis |
| HandoffV1 | Versioned JSON schema for state that may cross a switch |
| Dashboard | Local replay of recorded runs |

Reference harnesses in this repo exist so checkpoint and resume can be tested. External arms (mini-swe-agent, Terminus-2, later Claude Code and Codex CLI) are the comparison that counts.

## Evaluation

Run a zero-cost analysis of public Harbor trials before any paid job. Net headroom is the cross-harness oracle minus the same-harness best-of-*k* null. Gate 1 also requires cost at non-inferior success.

Then, through Harbor: Terminal-Bench 2.0 (pilot 20 tasks, then 89 × 3 arms × 3 seeds), one SWE-style adapter split by repository, Aider Polyglot, Harbor-Index. Specs are in [`veyra/docs/OBJECTIVE.md`](veyra/docs/OBJECTIVE.md) and [`veyra/docs/BENCHMARKS.md`](veyra/docs/BENCHMARKS.md).

## License

Apache-2.0. See [`veyra/LICENSE`](veyra/LICENSE).
