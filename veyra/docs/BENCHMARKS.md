# Benchmarks

The objective and the order of proof are in `OBJECTIVE.md`. Nothing in this file is a measured Veyra score.

## Status

| Benchmark | Role | Status |
|---|---|---|
| Bundled dev split with Kev-0.8B (`out/kev`) | Scripted tasks, live Kev decisions. | Measured. Heuristic 100% at $0.0096. Kev-0.8B 80% at $0.0386. Kev did not switch on recovery. Not MCPAgentBench. |
| Gemma 3 4B probe (`out/live-wide`) | Live model, laptop, dev tasks only. | Partial. The API dropped mid-run. |
| MCPAgentBench | Main external benchmark. Official checkout in `third_party/MCPAgentBench`. | Not run. Needs `ROUTER_API_KEY` and `veyra mcpagentbench --agent react --model …`. |
| BFCL v4 | Tool-selection baseline. | Not run. |
| τ-bench / τ³-bench | Stateful tool-agent-user. | Not run. |
| ToolSandbox | Adaptation and recovery. | Not run. |
| Veyra-ShiftBench | Our benchmark. Online shifts under a budget. | Specified. Not run as a public eval. |
| GAIA | Later broad-agent test. | Not scheduled. |
| SWE-bench | Later generality check, via the mini-swe adapter. | Not scheduled. |

## Running the Kev arm

Kev is the decision-model arm. It is not in the default offline suite, because a missing server must not be reported as a model result.

```bash
# The PyPI package named kev is unrelated. Install Jared Palmer's Kev from
# github.com/jaredpalmer/kev and serve it (default port 8009).
veyra compare --spawn --split dev --out out/kev \
  --arms fixed:native,veyra:heuristic,veyra:kev \
  --kev-endpoint http://127.0.0.1:8009
```

The event log labels the backend `kev`. If the endpoint is missing, the kernel records `decision_fallback` and the table is not a Kev result.

## Arms and metrics

Every public run uses the same arms: fixed ReAct, static tool router, Veyra heuristic, Veyra decision model, Veyra decision model plus bandit. Hold the model fixed across arms.

Metrics: task success, tool-selection accuracy, wasted tool calls, cost, latency, recovery after a shift, unsafe actions, abstentions.

## Veyra-ShiftBench

One episode. The tool catalog changes at a known step. The agent is not told the name of the shift. Scenarios:

| Shift | What changes |
|---|---|
| `unavailable` | The tool that worked returns a transport error. |
| `expensive` | The same tool still works, and its price jumps past the remaining budget. |
| `unreliable` | The tool starts failing on a fixed fraction of calls. |
| `schema` | A required argument is renamed. The old call fails validation. |
| `permission` | The credential for that tool is revoked. |
| `new_tool` | A cheaper tool that can finish the task appears. |
| `misleading` | The tool returns success with a wrong payload. The grader checks the artifact, not the tool's own status. |

Primary metric: successful completion after the shift, divided by extra dollars and extra calls spent after the shift. A run that keeps retrying the broken tool fails this metric even if it eventually succeeds.

## Where the public suites live

These are upstream projects. Veyra does not vendor them.

- MCPAgentBench: the paper's released tasks and the MCP servers they ship.
- BFCL v4: the Gorilla / Berkeley function-calling leaderboard, multi-turn and agentic splits.
- τ-bench: the Sierra Research suite and its current τ³ release.
- ToolSandbox: Apple's stateful tool-sandbox tasks.
- GAIA and SWE-bench: only after the four tool benchmarks above have a recorded arm.

A run is recorded only when the command, the model id, the arm, and the upstream commit are written next to the numbers.
