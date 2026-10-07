# Veyra

**Vendor-Neutral Tool-Boundary Reliability and Resolution Layer for AI Agents**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Python](https://img.shields.io/badge/python-3.10%2B-blue.svg)](python/)
[![Go](https://img.shields.io/badge/go-1.22%2B-cyan.svg)](go/)
[![Tests](https://img.shields.io/badge/tests-181%20passed-brightgreen.svg)](python/tests/)

```text
Existing Agent
     │  proposes action
     ▼
┌────────────────────────┐
│         VEYRA          │
│                        │
│ • Schema validation    │
│ • Parameter aliasing   │
│ • Equivalence mapping  │
│ • Safe fallbacks       │
│ • Failure taxonomy     │
│ • Idempotency guard    │
└───────────┬────────────┘
            │  resolves & executes
            ▼
   Tools / MCP Servers / APIs
```

> **The Core Abstraction**:  
> **Agent proposes → Veyra resolves → Tool executes.**

Veyra does **not** replace the agent's reasoning loop, planning, or conversation state.  
It is **not** a generic LLM router, SaaS gateway, or agent framework.

Veyra sits strictly at the agent/tool boundary. Its distinctive capability is **execution-resolution and continuity when the proposed tool path fails**: resolving parameter aliases, safe schema coercions, equivalent tool substitution, and declared fallback chains.

---

## 1. Safety Invariants

Veyra is engineered around non-negotiable safety constraints:

1. **Never semantically guess arguments**: Coercions must be provably type-safe or explicitly declared via alias maps.
2. **Never expand the allowed action set**: Resolutions are strictly bounded by policy-allowed tool definitions.
3. **Never substitute undeclared side-effecting tools**: Mutation tools (`write`, `delete`, `post`, `patch`) are never swapped speculatively.
4. **Never retry uncertain-state writes**: Non-idempotent operations are never retried blindly without strict idempotency confirmations.
5. **Never bypass authorization or permissions**: Boundary policy decisions (`SELECT`, `DEFER`, `DENY`) enforce access control.
6. **Zero LLM dependency**: Core deterministic resolution and adaptive execution memory operate with zero external model calls, zero prompt overhead, and sub-millisecond latency.

---

## 2. Empirical Benchmark Evidence: ContinuityBench

Veyra is evaluated under a **falsification-first protocol** on **`Veyra-ContinuityBench-v1.0`**—a paired perturbation benchmark (Clean vs. Perturbed execution) spanning 120 tasks across Repair, Gate, and strictly held-out Scorecard splits.

Crucially, Veyra is benchmarked against **`static_resolution`**—a fair, hard competitor receiving the **EXACT SAME** equivalence declarations, argument aliases, fallback candidates, and hard safety constraints.

### A. Held-Out Scorecard Results (40 Paired Perturbation Tasks)

| System Arm | Clean-Task Success | Perturbed Success | Intent Preservation Rate (IPR) | Degradation ($\Delta$) | Replan Rate | Unsafe Substitutions |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: |
| `raw_agent` | 100.0% | 0.0% | **0.0%** | 100.0% | 100.0% | 0 |
| `competent_boundary` | 100.0% | 0.0% | **0.0%** | 100.0% | 100.0% | 0 |
| `static_resolution` | 100.0% | 100.0% | **80.0%** | 0.0% | 0.0% | 0 |
| **`veyra` (Contract-Aware)** | **100.0%** | **100.0%** | **100.0%** | **0.0%** | **0.0%** | **0** |
| `oracle` (Sanity Bound) | 100.0% | 100.0% | **100.0%** | 0.0% | 0.0% | 0 |

**Why `static_resolution` fails on 20% of cases**:
Under state and freshness constraints (Perturbations G and I), `static_resolution` blindly picks the first candidate in the fallback list, executing stale endpoints (e.g. freshness 45s > 10s contract limit). Veyra evaluates the **`ExecutionContract`**, detects the invariant violation, filters out stale candidates, and selects the contract-preserving replica, delivering a **+20.0 percentage-point absolute lift**.

### B. Held-Out TAGE Hypothesis Validation (Scorecard Split)

| Resolution Policy | Held-Out IPR | Unsafe Explorations | Decision Latency |
| :--- | :---: | :---: | :---: |
| `static_resolution` | 80.0% | 0 | 0.1 $\mu$s |
| `case_based_memory` | **100.0%** | **0** | 0.1 $\mu$s |
| **`tage_history`** | **100.0%** | **0** | **0.1 $\mu$s** |
| `linucb_bandit` | **100.0%** | 0 | 597.4 $\mu$s (~6,000x slower) |

*Finding*: Multi-history execution memory (`tage_history`) matches contextual bandits on unseen states at **sub-microsecond latency** (0.1 $\mu$s) with zero unsafe exploration hazards.

### C. External Benchmark Perturbations & Real LLM Pilot

- **External Generalization**: Validated on 25-task subsets of **Tau2-Bench Verified**, **BFCL Multi-Turn**, and **MCPMark Verified** (+20.0pp IPR lift across all three).
- **Real LLM Pilot (270 Trajectories: 30 tasks $\times$ 3 arms $\times$ 3 seeds)**: Veyra achieved **100% recovery** (+20pp over static), **0 replans**, **-50.0% agent turns**, and **-48.6% prompt token cost** vs. raw agent.
- **Decision Gates A–E**: All 5 decision gates (Resolution Capability, Real Agent Effect, External Transfer, Adaptive Value, Product Value) evaluated and **PASSED**. Full report in [`benchmarks/continuitybench/REPORT_CONTINUITYBENCH.md`](benchmarks/continuitybench/REPORT_CONTINUITYBENCH.md).

---

## 3. Installation

```bash
# Install Python SDK
pip install -e python/

# Build Go Kernel CLI
cd go && go build -o bin/veyra ./cmd/veyra
```

---

## 4. Quickstart

### A. Python SDK (Code Configuration)

```python
from veyra import Veyra, ToolRegistry, ToolDefinition

registry = ToolRegistry()

# Register healthy fallback
registry.register(ToolDefinition(
    name="backup_customer_service",
    executable=lambda customer_id: {"id": customer_id, "name": "Jane Doe"},
    idempotent=True,
))

# Declare equivalence, alias remapping, and fallback chain
registry.register_equivalence("primary_crm", ["backup_customer_service"])
registry.register_parameter_aliases("backup_customer_service", {"uid": "customer_id"})
registry.register_fallback_chain("primary_crm", ["backup_customer_service"])

# Initialize boundary middleware
veyra = Veyra(registry=registry)

# Agent proposes call to primary with parameter alias 'uid'
result = veyra.call(
    tool_name="primary_crm",
    arguments={"uid": 1001},
    idempotent=True,
)
print(result)
# Output: {'id': 1001, 'name': 'Jane Doe'}
```

### B. Declarative YAML Configuration

Define capabilities, parameter aliases, and fallback chains without touching code:

```yaml
# config/capabilities.yaml
version: "1.0"
capabilities:
  customer_lookup:
    tools:
      - primary_crm
      - backup_customer_service
    primary_tool: primary_crm
    parameter_aliases:
      primary_crm:
        uid: customer_id
      backup_customer_service:
        uid: customer_id
    fallback_chains:
      primary_crm:
        - backup_customer_service
    idempotent: true
    risk_class: low
```

Load in Python:

```python
from veyra import Veyra, ToolRegistry, load_equivalence_config

cfg = load_equivalence_config("config/capabilities.yaml")
registry = ToolRegistry()
cfg.apply_to_registry(registry)

veyra = Veyra(registry=registry)
```

### C. Model Context Protocol (MCP) Middleware

Wrap any MCP server handler transparently:

```python
from veyra import MCPToolMiddleware, Veyra

middleware = MCPToolMiddleware(veyra=Veyra())

# Intercepts MCP tools/call JSON-RPC requests
response = middleware.handle_call_tool(
    tool_name="query_database",
    arguments={"query": "SELECT * FROM users", "limit": "50"},
    handler=my_mcp_database_handler,
    input_schema=database_tool_schema,
    idempotent=True,
)
```

---

## 5. Repository Structure

```text
├── benchmarks/
│   └── resolutionbench/        # Dedicated 100-task ground-truth benchmark
│       ├── cases.json          # 100 hand-crafted tasks across Categories A-E
│       ├── evaluator.py        # 4-arm comparative benchmark runner
│       ├── REPORT_PHASE1_PHASE2.md
│       ├── REPORT_PHASE3.md    # Strategy lab results
│       ├── REPORT_PHASE4.md    # TAGE history results
│       ├── REPORT_PHASE5.md    # Beta reliability results
│       ├── REPORT_PHASE6.md    # Calibrated selective resolution
│       ├── REPORT_PHASE7.md    # Real agent pilot report
│       ├── REPORT_PHASE8.md    # Real MCP validation report
│       └── REPORT_PHASE9.md    # Learning & OPE falsification report
├── examples/
│   ├── 01_quickstart_deterministic.py
│   ├── 02_mcp_middleware.py
│   ├── 03_adaptive_history_tage.py
│   ├── 04_declarative_config.py
│   └── 04_declarative_config.yaml
├── go/                         # High-performance Go kernel
│   ├── cmd/veyra/              # CLI entry point
│   └── internal/               # Policy engine & task scheduler
└── python/
    └── src/veyra/
        ├── boundary/           # Interceptor, MCP middleware, failure taxonomy
        ├── config/             # Declarative YAML/JSON equivalence loader
        ├── core/               # ExecutableAction, Decision, State, Trajectory
        ├── execution/          # ExecutionEngine and runtime coordinator
        ├── export/             # StructuredTraceExporter (JSONL, OTel, metrics)
        ├── lab/                # Resolution Strategy Lab plugins (BM25, TAGE, etc.)
        ├── plugins/            # RoutePolicy plugin registry
        ├── policy/             # Deterministic, TAGE, Beta-Reliability, Selective, Bandits
        └── registry/           # ToolRegistry and CandidateResolver
```

---

## 6. Standardized Machine-Readable Trajectory Logging

Every call through Veyra records 17 standardized trajectory fields:

```json
{
  "task_id": "res_a_01",
  "arm": "veyra_deterministic",
  "turn_index": 1,
  "proposed_action": {"tool": "fetch_customer", "arguments": {"customer_id": 1001}},
  "candidate_actions": [{"tool": "get_customer"}],
  "selected_action": {"tool": "get_customer", "arguments": {"customer_id": 1001}},
  "resolution_reason": "deterministic resolution via equivalence and fallback chain",
  "policy_decision": "SELECT",
  "failure_kind": null,
  "failure_provenance": null,
  "retry_count": 0,
  "recovery_action": "fallback_and_coerced",
  "agent_replan": false,
  "tool_result": {"status": "success"},
  "final_success": true,
  "tokens": {"prompt": 450, "completion": 50, "total": 500},
  "latency": 0.0085
}
```

Failure provenance distinguishes 10 frozen values:
- `AGENT_ARGUMENT_ERROR`
- `SCHEMA_VALIDATION_ERROR`
- `TOOL_IMPLEMENTATION_ERROR`
- `NETWORK_ERROR`
- `RATE_LIMIT`
- `TIMEOUT`
- `AUTHORIZATION_ERROR`
- `PRECONDITION_ERROR`
- `UNKNOWN_STATE`
- `UNKNOWN`

---

## 7. License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
