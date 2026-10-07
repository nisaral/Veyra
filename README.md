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

## 2. Empirical Benchmark Evidence

Veyra was evaluated under a **falsification-first protocol** across four distinct benchmarks and pilots:

### A. ResolutionBench v0 (100 Hand-Crafted Ground-Truth Tasks)
*Evaluates alternate tool resolution, parameter aliases, schema drift, declared fallback chains, and compound failures.*

| Benchmark Arm | Category A (Equiv) | Category B (Aliases) | Category C (Schema) | Category D (Fallback) | Category E (Compound) | Overall Recovery | Unsafe Substitutions | Latency |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| `raw_llm` | 0.0% (0/20) | 0.0% (0/20) | 0.0% (0/20) | 0.0% (0/20) | 0.0% (0/20) | **0.0% (0/100)** | 0 | 0.01 ms |
| `competent_baseline` | 0.0% (0/20) | 0.0% (0/20) | 100.0% (20/20) | 0.0% (0/20) | 0.0% (0/20) | **20.0% (20/100)** | 0 | 0.01 ms |
| **`veyra_deterministic`** | **100.0% (20/20)** | **100.0% (20/20)** | **100.0% (20/20)** | **100.0% (20/20)** | **100.0% (20/20)** | **100.0% (100/100)** | **0** | **8.58 ms** |
| `oracle` (Sanity Ceiling) | 100.0% (20/20) | 100.0% (20/20) | 100.0% (20/20) | 100.0% (20/20) | 100.0% (20/20) | **100.0% (100/100)** | 0 | 0.01 ms |

*Result*: **+80.0 percentage-point absolute improvement** over a competent middleware baseline with **zero unsafe substitutions** and **zero unauthorized actions**.

### B. Real Agent Pilot (20 Paired Tasks $\times$ 5 Random Seeds)
*Evaluates boundary resolution against an autonomous ReAct loop on tool failure tasks.*

- **Agent Re-plans**: Eliminated by **100.0%** (25 replans $\to$ **0 replans**).
- **Model Turns**: Reduced by **-26.7%** (75 turns $\to$ 55 turns).
- **Prompt Token Savings**: Saved **~10,000 prompt tokens** on recoverable timeouts (-52.6% token cost).
- **Unsafe Retries**: **0** unsafe operations committed.

### C. Real FastMCP Validation (40 Controlled Tasks across 3 Real MCP Servers)
*Evaluates interoperability across real Filesystem, Database, and API FastMCP servers.*

- **Task Success**: **65.0%** (+30.0 pp over raw agent).
- **Boundary Fault Recovery**: **100.0%** (16/16 eligible real-world faults recovered).
- **Safety Comparison**: `naive_retry` committed **10 data-corrupting retries** on non-idempotent operations; `veyra` committed **0**.
- **Clean-Task Non-Regression**: 100% direct pass-through on non-faulted tasks with 1.30 ms mean latency.

### D. Offline Strategy Lab & Off-Policy Evaluation (OPE)
*Falsification test: Does online reinforcement learning or contextual bandits justify operational complexity?*

- **Finding**: Multi-history execution memory (`tage_history`, $H_0, H_1, H_2, H_4, H_8$) and Bradley-Terry ranking achieve identical reward estimation (**DR = 0.6387**) to LinUCB contextual bandits.
- **Latency Advantage**: TAGE memory executes in **1.1 $\mu$s**, which is **50x faster** than bandit matrix inversions (48–64 $\mu$s).
- **Conclusion**: Complex RL/online exploration is **not justified** at the execution boundary. Veyra ships with deterministic equivalence and TAGE execution memory.

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
