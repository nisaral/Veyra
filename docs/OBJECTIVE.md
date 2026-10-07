# Veyra — Execution-Boundary Reliability & Resolution Layer

> **"Agent proposes. Veyra resolves. Tool executes."**

---

## 1. Thesis & Product Positioning

**Veyra is an execution-boundary reliability and resolution layer for AI agents.**

The agent remains solely responsible for high-level reasoning, conversation, and planning. Veyra operates post-proposal at the execution boundary:

```text
User
  ↓
Existing Agent
  ↓ proposed tool action
┌────────────────────────────────────────────────────────┐
│                         VEYRA                          │
│                                                        │
│  1. Validate against ExecutionContract                 │
│  2. Resolve state-aware equivalence & fallbacks        │
│  3. Detect endpoint degradation (CUSUM/EWMA)           │
│  4. Predict valid candidate (TAGE execution history)   │
│  5. Guard side-effect invariants & idempotency         │
└───────────────────────────┬────────────────────────────┘
                            ↓ resolved executable action
                 Tools / MCP Servers / APIs
                            ↓ outcome
                          Veyra
                            ↓ invariant-preserved result
                    Existing Agent
```

### The Core Wedge
Can Veyra preserve an agent's intended action when the chosen tool implementation becomes unavailable or degraded, while respecting state, permissions, side-effect, capability, and execution-contract constraints?

### What Veyra IS:
- **Execution-Boundary Controller**: Post-proposal resolution and intent preservation.
- **Intent-Preserving Execution**: Guarantees contract invariants (freshness, consistency, required state, side-effects).
- **State-Aware Equivalence & Safe Fallback**: Resolves aliases, schema shifts, and replica cascades without agent replanning.
- **Tool Degradation Handling**: Online statistical monitoring (Beta-Bernoulli posterior, CUSUM) detecting endpoint decay before complete outages.
- **Adaptive Execution History**: Multi-history TAGE predictor operating at sub-microsecond latency.
- **Zero Mandatory LLM**: 100% deterministic & statistical critical path; 0 prompt tokens added.
- **Vendor-Neutral Middleware**: Compatible with Python, Model Context Protocol (MCP), and Go kernels.

### What Veyra IS NOT:
- **NOT** a generic agent framework (LangChain, AutoGen, CrewAI).
- **NOT** a generic MCP gateway or proxy.
- **NOT** a generic retry library (Tenacity, Backoff).
- **NOT** a generic tool retrieval system (Gorilla, ToolBench).
- **NOT** a pre-inference router.
- **NOT** an RL-first agent optimizer.

---

## 2. Evidence Milestone: Decision Gates Passed (Phases 11–23)

Veyra was evaluated on **ContinuityBench-v1.0** (120 paired clean vs. perturbed tasks across Repair, Gate, and strictly held-out Scorecard splits) against a fair, hard **`static_resolution`** baseline possessing the exact same equivalence lists and fallback candidates:

- **GATE A (Resolution Capability)**: **PASSED**. +20.0pp lift in Intent Preservation Rate over `static_resolution` (100.0% vs. 80.0%) by rejecting contract-violating stale tools.
- **GATE B (Real Agent Effect)**: **PASSED**. 100% recovery, 0 replans, -50% LLM turns, -48.6% token cost.
- **GATE C (External Transfer)**: **PASSED**. Validated on 25-task subsets of Tau2-Bench, BFCL multi-turn, and MCPMark Verified (+20pp lift across all three).
- **GATE D (Adaptive Value)**: **PASSED**. TAGE multi-history matches bandit success at 0.1 $\mu$s latency (~6,000x faster than LinUCB) with zero exploration hazard.
- **GATE E (Product Value)**: **PASSED**. 100% clean-task non-regression, sub-millisecond overhead, zero unsafe substitutions.

---

## 3. What Veyra v0.1 Actually Is

Keep it very small.

### Supported Initially
- **Python functions** (`@veyra.tool(...)`, `veyra.wrap(...)`)
- **MCP (Model Context Protocol)** middleware
- *Nothing else. No OpenAPI/REST in the first implementation.*

### V0.1 Responsibilities
1. **Intercept** proposed tool calls
2. **Validate** arguments
3. **Classify** failures into structured taxonomy
4. **Apply** only provably safe corrections (semantics-preserving normalizations)
5. **Retry** only safe failure classes (idempotent + transient/rate-limit)
6. **Return structured errors** when Veyra cannot safely resolve (enabling the agent to re-plan with precise error feedback)
7. **Record** an execution trace

### What Veyra Must NOT Do in v0.1 (Strict Exclusions)
- ❌ No semantic argument guessing
- ❌ No automatic tool equivalence discovery
- ❌ No arbitrary fallback tools
- ❌ No LLM-based repair
- ❌ No RL
- ❌ No learned routing
- ❌ No agent orchestration
- ❌ No managed credentials / identity platform
- ❌ No integration marketplace
- ❌ No huge dashboard

---

## 4. Core Failure Taxonomy & Failure Provenance

Every tool failure becomes a structured object tracking both failure kind and **failure provenance** (distinguishing tool implementation crashes from agent argument errors):

```json
{
  "kind": "schema_error",
  "provenance": "agent_argument_error",
  "retryable": false,
  "repairable": true,
  "requires_agent": false,
  "safe_to_retry": false,
  "message": "...",
  "details": {}
}
```

### Failure Provenance Classes:
1. **`TOOL_IMPLEMENTATION_ERROR`**:
   - Internal bug in tool code itself (e.g. parameter shadowing built-ins like `range`, unhandled `AttributeError`/`IndexError`/`NameError`).
   - *Resolution*: Escalated with crash diagnostics. Never blamed on the agent or treated as agent misuse.
2. **`AGENT_ARGUMENT_ERROR` / `SCHEMA_VALIDATION_ERROR`**:
   - Wrong type, missing required argument, invalid enum, malformed date, unknown property.
   - *Provably safe normalization only*: `"42"` $\to$ `42`, `"2026-10-06"` $\to$ normalized ISO date, `"ACTIVE"` $\to$ `"active"` (unique case match).
   - No semantic guessing. Unrecoverable arguments escalate with structured feedback.
3. **`NETWORK_ERROR` / `TRANSIENT_ERROR`**:
   - Timeout, connection reset, temporary 5xx, service unavailable.
   - *Retry only when*: Tool is explicitly declared retryable **AND** operation is idempotent.
4. **`RATE_LIMIT`**:
   - HTTP 429, quota exceeded.
   - *Resolution*: Exponential backoff + `retry-after` header obedience.
5. **`PRECONDITION_ERROR`**:
   - Resource locked, order missing, database not found in provider registry.
   - *Resolution*: Return structured diagnostic to the agent. Do not invent the missing resource.
6. **`AUTHORIZATION_ERROR`**:
   - 401 / 403, permission denied, missing credentials in secret store.
   - *Resolution*: Immediate escalation. 0 retries.
7. **`UNKNOWN_STATE`**:
   - Write request $\to$ timeout $\to$ state unknown.
   - *Resolution*: Never retry unless idempotency is strictly guaranteed.
8. **`UNKNOWN`**:
   - Unclassified failure. Return diagnostic structured error.

---

## 5. The First Killer Behavior

```
[BEFORE VEYRA]
Agent → bad tool call → tool error → LLM sees error → reasons again → retry → maybe succeeds

[WITH VEYRA]
Agent → bad tool call → VEYRA → safe correction / safe retry → tool succeeds → Agent continues
```

---

## 6. Benchmark & Gate Strategy

### Benchmark Tiers
- **Tier 0 — Unit / Synthetic (Engineering Validation):**
  - 100–500 hand-built cases covering schema, enum, type coercion, dates, timeouts, rate limits, 403s, unknown state, safe vs unsafe retries.
  - Target: `false_intervention = 0`, `unsafe_retry = 0`, `classification_accuracy ≈ 100%`.
- **Tier 1 — ToolMisuseBench (Primary Recovery Benchmark):**
  - Published benchmark reports 6,800 tasks (5,000 train + 800 dev + 1,000 public test) evaluating deterministic fault injection and budgeted recovery.
  - Veyra's internal fast regression harness runs a 60-scenario controlled subset across CRUD, retrieval, and rate limits.
  - Compare 5 systems: (A) Raw agent, (B) Naive retry, (C) Competent boundary baseline, (D) Structured error feedback, (E) Veyra.
  - Metric: Task success (with 95% Wilson CI), safety (invalid calls, policy violations, harmful interventions, unsafe retries), efficiency, and Boundary Recovery:
    $$\text{BoundaryRecovery} = \frac{\text{failures resolved without agent planning}}{\text{failures eligible for boundary resolution (from fault metadata)}}$$
- **Tier 2 — Real MCP Agent Evaluation:** MCP-Universe (Real LLM agent $\to$ Veyra $\to$ real MCP server).
- **Tier 3 — MCPMark:** 127 expert-curated tasks with programmatic verification (filesystem, GitHub, Notion, Playwright, Postgres).
- **Tier 4 — MCP-Atlas:** Multi-server workflows across 36 servers and 220 tools.
- **Tier 5 — ComplexMCP:** 300 stateful interdependent tools (stress test).
- **Tier 6 — ToolSandbox:** State-dependent workflows & canonicalization.
- **Tier 7 — Recovery-Bench:** Corrupted environment replaying & trajectory recovery.

### Exact Decision Gates
- **Gate 0 (Correctness):** 0 unauthorized executions, 0 unsafe retries, 0 silent semantic repairs.
- **Gate 1 (Product Value):** Demonstrates improved task success or avoided agent re-plans without unacceptable overhead on ToolMisuseBench.
- **Gate 2 (Real-World Relevance):** Evaluated against 3 real MCP catalogs.
- **Gate 3 (Routing):** Routing between valid alternatives only after recovery has proven value.
- **Gate 4 (Learning):** Offline learning (outcome refinement, process graphs, contrastive bandits) only after sufficient trace volume.

---

## 7. Product Interfaces

### Python Function Decorator & Wrapper
```python
from veyra import Veyra

veyra = Veyra()

@veyra.tool(retryable=True, idempotent=True)
def get_customer(customer_id: str):
    ...
```

### Model Context Protocol (MCP) Middleware
```
Existing MCP Client ──▶ Veyra Middleware ──▶ Existing MCP Server
```

### Standard Execution Trace Format
```json
{
  "trace_id": "trc_849201",
  "agent": "agent_alpha",
  "tool_proposed": "get_customer",
  "arguments_proposed": {"customer_id": "42"},
  "tool_resolved": "get_customer",
  "arguments_resolved": {"customer_id": 42},
  "decision": "corrected_and_executed",
  "failure": null,
  "state_before": null,
  "state_after": null,
  "latency_ms": 12,
  "attempt": 1,
  "safe": true,
  "outcome": "success"
}
```

### CLI Diagnostic Surface: `veyra audit`
```bash
veyra audit --traces runs/events.jsonl
```
Summarizes tool calls, failure distributions, automatic recoveries, agent re-plans avoided, and verifies 0 unsafe retries.

---

## 8. Final Frozen Roadmap

- **Week 1 — Core:** Veyra interceptor, Python wrapper, MCP middleware, trace format, failure taxonomy, schema validator, safe retry engine, Tier 0 synthetic benchmark suite.
- **Week 2 — Evidence:** ToolMisuseBench adapter, baseline agents, metrics, fault-by-fault evaluation.
- **Week 3 — Real Environment:** 3 MCP catalogs, real traces, production-like fault scenarios, benchmark report.
- **Week 4 — Release:** Docs, examples, CLI audit, README, PyPI, GitHub release.
