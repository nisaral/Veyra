# Veyra

**Open-source reliability middleware for tool-using AI agents.**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Python](https://img.shields.io/badge/python-3.10%2B-blue.svg)](python/)
[![Go](https://img.shields.io/badge/go-1.22%2B-cyan.svg)](go/)
[![Tests](https://img.shields.io/badge/tests-213%20passed-brightgreen.svg)](python/tests/)

> *"Make tool execution safer without rewriting your agent."*

```text
Existing Agent
     │  proposes action
     ▼
┌──────────────────────────────────────────────┐
│               VEYRA MIDDLEWARE               │
│                                              │
│  • Execution contract & state validation     │
│  • Unknown-state & dropped ACK verification  │
│  • Deterministic fallback candidate selector │
│  • Circuit breaker & bounded transient retry│
│  • Cryptographic audit hash-chain            │
└──────────────────────┬───────────────────────┘
                       │  resolves & executes
                       ▼
            Tools / MCP Servers / APIs
```

> **The Core Abstraction**:  
> **Agent proposes → Veyra validates and resolves → Tool executes.**

Veyra is **not** an agent framework, pre-inference router, or LLM wrapper.  
It is drop-in reliability middleware positioned strictly at the execution boundary between the agent and its tools.

---

## ⚡ Quickstart: Drop-In Middleware (<2 Minutes)

Install and protect any tool or MCP client without touching the agent's reasoning loop:

```python
from veyra import VeyraMiddleware, Mode, SideEffectClass

middleware = VeyraMiddleware()

# 1. Protect any Python tool
safe_transfer = middleware.wrap_function(
    bank_transfer,
    name="bank_transfer",
    side_effect_class=SideEffectClass.NON_IDEMPOTENT_MUTATION,
    verification_fn=check_transfer_status,
)

# 2. Or wrap an entire MCP Client
wrapped_mcp = middleware.wrap_mcp(my_mcp_client)

# Your agent remains completely unchanged:
# agent = ExistingAgent(tools=[safe_transfer])
```

### Supported Modes:
- **`NORMAL`**: Full boundary interception, contract validation, and recovery.
- **`SHADOW`**: Observes live executions and logs decisions without modifying calls.
- **`DRY_RUN`**: Returns structured decision explanations and contract checks without executing.
- **`FAIL_CLOSED` / `FAIL_OPEN`**: Configurable fallback safety policy on internal errors.

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

## 2. Killer Demo: Unknown-State Safety (UNKNOWN_ACK)

Consider the most dangerous failure mode in AI agent operations:
1. Agent proposes non-idempotent tool (e.g. `$50,000` bank wire or server deletion).
2. The mutation successfully commits in the target service, but the network drops or times out before the client receives the ACK (**`UNKNOWN_ACK`**).
3. **Naive Agent / Standard Retry**: Assumes failure, retries the request $\rightarrow$ **Duplicate external mutation ($100,000 transferred)**!
4. **Veyra Execution Boundary**:
   - Intercepts `UNKNOWN_ACK` on non-idempotent mutation.
   - Enforces core safety invariant: **NEVER blind replay**.
   - Transitions to `VERIFY` $\rightarrow$ queries idempotent verification oracle $\rightarrow$ discovers committed record $\rightarrow$ returns verified success without duplicating the mutation!

```bash
# Run the killer demo directly:
python examples/unknown_ack/main.py
```

---

## 3. Evidence Status & Controlled Validation

*Note on Evidence Classification*: Internal controlled evidence confirms Veyra's invariant protection, property tests, dynamic state resolution, and zero-duplicate transaction safety. In accordance with our open-source release principles, all external benchmark numbers remain quarantined as `PROVISIONAL_UNVERIFIED` until reproduced via independent audit runners. See [`EVIDENCE_STATUS.md`](benchmarks/final_evidence/EVIDENCE_STATUS.md).

### Controlled Invariants Confirmed:
- **0 Duplicate Mutations**: Strict rejection of blind replays under `UNKNOWN_ACK` and `PARTIAL`.
- **0 Side-Effect Widening**: Strict prevention of mutating replacements when read-only requested.
- **0 Unauthorized Operations**: Tenant isolation and permission enforcement.
- **Microsecond Policy Overhead**: <0.05 ms boundary resolution overhead compared to typical network tool latencies (100–300 ms).


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
