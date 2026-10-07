# Veyra — Execution Control Middleware for Tool-Using AI Agents

> **Your agent decides what it wants to do. Veyra makes sure the action is still safe and executable.**
>
> Add reliability at the execution boundary without rewriting your agent.

---

## 1. Problem

When AI agents invoke tools (databases, APIs, payment gateways, shell scripts), non-deterministic LLM planning often results in:
- Executing unauthorized or dangerous parameters.
- Re-executing non-idempotent mutations after network drops (`UNKNOWN_ACK`).
- Calling stale or degraded endpoints.
- Broken schema serialization and missing parameters.

Existing agent retry logic blindly replays requests, risking duplicate mutations (e.g. charging a customer twice).

## 2. Why Existing Agent Retry / Tool Routing Is Insufficient

Standard retry handlers and LLM routers operate **pre-inference** or rely on LLM re-prompting. They lack deterministic execution contract safety, state awareness, and idempotency guarantees.

Veyra sits **post-proposal, pre-execution** at the execution boundary:
```text
Agent Proposes Action → Veyra Validates/Resolves → Tool Executes
```

---

## 3. 30-Second Install

```bash
pip install veyra
```

Verify installation:
```bash
veyra doctor
```

---

## 4. 10-Line Integration

```python
from veyra import Veyra

veyra = Veyra(policy="default", mode="fail_closed")

@veyra.wrap
def process_refund(customer_id: str, amount: float) -> dict:
    return {"status": "refunded", "customer": customer_id, "amount": amount}

# The agent proposes a tool call; Veyra enforces contract safety & idempotency
result = process_refund(customer_id="cust_123", amount=50.0)
print(result)
```

---

## 5. UNKNOWN_ACK Safe Handling

```python
from veyra import Veyra
from veyra.core.action import ExecutableAction

veyra = Veyra(mode="fail_closed")

# Non-idempotent mutation during network drop receives UNKNOWN_ACK
# Veyra requires verification before replay — preventing blind duplicates
```

---

## 6. MCP Proxy Demo

Place Veyra between any MCP Client (Agent) and MCP Server without modifying your agent code:

```bash
veyra proxy mcp
```

Architecture:
```text
Agent (MCP Client) → Veyra MCP Proxy → Veyra Policy/Resolution → Real MCP Server
```

---

## 7. Policy Example

```yaml
# veyra.yaml
veyra:
  mode: fail_closed

policy:
  tenant_isolation: true
  authorization: true
  freshness: true
  health: true

recovery:
  unknown_ack: verify_then_defer
  retries:
    enabled: true
    max_attempts: 2
```

Validate & explain configuration:
```bash
veyra config validate
veyra config explain
```

---

## 8. Trace and Replay Example

Replay an execution trajectory trace without executing side effects (DRY_RUN):
```bash
veyra replay run.json
```

Opt-in to live execution:
```bash
veyra replay run.json --execute
```

Diff two run trajectories:
```bash
veyra diff run1.json run2.json
```

---

## 9. Benchmark Results

Controlled evaluation results demonstrate Veyra's deterministic execution resolution:

| Evaluation Suite | Control Baseline | Veyra Execution Control |
|---|---|---|
| **Contract Invariants** | Unsafe side effects | 100% Safety Enforcement |
| **UNKNOWN_ACK Mutations** | Blind replay duplicates | Zero Blind Replay Duplicates |
| **UndoBench DER** | 100% Failure Rate | 0% DER (Execution Safety) |

---

## 10. Architecture

```text
LangGraph / OpenAI Agents / AutoGen / Microsoft Agent Framework
                         │
                       AGENT
                         │
                         ▼
                       VEYRA
                         │
                    execution
                         │
              MCP / Python / HTTP
```

---

## 11. Framework Integrations

Veyra provides first-party adapters for:
- **OpenAI Agents SDK**: `from veyra.integrations.openai_agents import VeyraOpenAIAgentsAdapter`
- **LangGraph**: `from veyra.integrations.langgraph import VeyraLangGraphNode`
- **AutoGen**: `from veyra.integrations.autogen import VeyraAutoGenAdapter`
- **Microsoft Agent Framework**: `from veyra.integrations.microsoft_agent_framework import VeyraMicrosoftAgentMiddleware`

---

## 12. Roadmap & License

- **v0.2.0**: Unified Python SDK, MCP Proxy, Framework Adapters, CLI, Trajectory Replay & Diffing, Plugin System.
- **License**: Apache 2.0.
