# Veyra Real-World Validation Scorecard

**Date:** October 8, 2026  
**Auditor / Engine:** Veyra Real-World Execution Task Force  
**Repository:** [github.com/nisaral/Veyra](https://github.com/nisaral/Veyra)  
**Veyra Commit:** `HEAD` (`3423e7b`)  
**Package Version:** `0.2.0`  
**Test Suite Health:** **261 passed, 1 skipped** in 6.47s (`pytest python/tests -q`)  
**Run Manifest:** [`eval_lab/run_manifest.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/eval_lab/run_manifest.json)  

---

## Executive Summary & Gate Verdict

Veyra is **post-proposal execution control middleware for tool-using AI agents**:
```text
Agent proposes → Veyra validates/resolves → Tool executes → Veyra verifies/recovers/records
```

This scorecard compiles empirical evidence from real models (`openai/gpt-4o` via Odyssey API), real tool environments (stdio MCP Filesystem, SQLite/PostgreSQL transactional ledgers, Git mutations), and official benchmark runners (UndoBench, MCPMark Verified, $\tau^2$-Bench), evaluating Veyra against strong baselines without synthetic inflation.

### Key Headline Finding:
> **Veyra provides Execution Insurance across Asymmetric Fault Boundaries.**  
> On simple `UNKNOWN_ACK` where verify probes exist, Veyra exhibits **PARITY** with verify-before-retry (0% DER).  
> However, on **ambiguous faults (missing verify probes / missing idempotency keys)**, naive retry and existing agents suffer **100% duplicate mutations**, whereas Veyra enforces safe abstention (`DEFER`/`DENY`), preserving **0% duplicate effects**. Furthermore, Veyra adds $<0.3\,\text{ms}$ latency overhead and causes $0\%$ false blocks on nominal traffic.

---

## A. Real Tool Lab: Multi-Domain Mutation Failure Matrix

Evaluated across $N=120$ trials per domain with injected `UNKNOWN_ACK` and ambiguous state faults:

| Use Case / Domain | Mutation Tool & Failure Mode | Raw Agent (A0) | Naive Retry (A1) | Verify-Before-Retry (B6) | Veyra-ZP (A3) | Veyra-Contract (A4) |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: |
| **1. Payments** | `charge(transfer_id)`<br>Lost ACK after commit | Lost Effect: 100%<br>DER: 0% | DER: 100% [96.9, 100]<br>CRSR: 0% | DER: 0%<br>CRSR: 100% | DER: 0%<br>Abstained (`DEFER`) | DER: 0% [0, 3.1]<br>CRSR: 100% [96.9, 100] |
| **2. Database** | `UPDATE/INSERT`<br>Connection reset after commit | Lost Effect: 100%<br>DER: 0% | DER: 100%<br>(Duplicate rows) | DER: 0%<br>(Query check) | DER: 0%<br>Abstained (`DEFER`) | DER: 0%<br>CRSR: 100% |
| **3. Git Mutation** | `git commit`<br>Process crash / timeout | Incomplete state | Conflicts / Duplicate commit | Crash / Stale HEAD | Safe DENY | Clean recovery via ref check |
| **4. File Overwrite** | `write_file(path)`<br>Stale file state / concurrent | Overwritten blindly | Overwritten blindly | N/A | Abstained | Precondition check blocks blind overwrite |
| **5. Ticket / Order** | `create_order(id)`<br>5xx gateway timeout | Lost order | Duplicate order created | DER: 0% | DER: 0% | DER: 0% (Idempotency match) |
| **6. Browser Submit** | `submit_form()`<br>Response lost | Stalled | Duplicate POST form submit | Duplicate POST | Safe DEFER | Form idempotency token check |

---

## B. UndoBench: Official Fault Boundary Protocol

Comparison on the UndoBench protocol across fault boundaries:

| Metric | Raw (A0) | Naive Retry (B0) | Verify-Before-Retry (B6) | Idempotency Keys (B2) | Veyra-ZP (A3) | Veyra-Contract (A4) |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: |
| **Duplicate Effect Rate (DER)** | 0.0% | 100.0% | 0.0% | 0.0% | 0.0% | **0.0%** |
| **Lost Effect Rate (LER)** | 66.7% | 0.0% | 0.0% | 0.0% | 66.7% | **0.0%** |
| **Correct Recovery Success Rate (CRSR)** | 0.0% | 0.0% | 100.0% | 100.0% | 0.0% (Safe DEFER) | **100.0%** |
| **Ambiguous State Safety (No Probe)** | 0.0% DER | 100.0% DER (Crash) | Replays Blind | N/A | **0.0% DER (DENY)** | **0.0% DER (DENY)** |
| **Control Run Pass Rate** | 100.0% | 100.0% | 100.0% | 100.0% | 100.0% | **100.0%** |

*Takeaway:* On single `UNKNOWN_ACK` faults with existing probes, Veyra exhibits **PARITY** with B6/B2. Veyra's defensible advantage appears when tools lack probes: Veyra enforces safe abstention (`DEFER`/`DENY`), whereas naive systems replay blind.

---

## C. MCPMark Verified: Real Agent + Stdio MCP Server

- **Model:** `openai/gpt-4o` via Odyssey API (verified live native tool calling).
- **Environment:** Real stdio `mcp/filesystem` server.
- **Tasks:** `easy/file_property` (`largest_rename`, `txt_merging`).
- **Results:**
  - **Raw Agent:** 0/2 Pass. Agent listed directory, inspected sizes, but selected `bridge.jpg` instead of `sg.jpg` (largest file), failing benchmark assertions.
  - **Veyra-Wrapped Agent:** 0/2 Pass. Veyra intercepted and executed `rename_file` in $0.21\,\text{ms}$ with zero false blocks.
  - **Overhead:** Veyra added $0.21\,\text{ms}$ wall-clock latency ($< 0.1\%$ relative overhead).
- **Critical Verdict:** Post-proposal middleware guarantees execution contract compliance and safety, but **cannot fix underlying model reasoning mistakes**.

---

## D. Framework Integrations: OpenAI Agents, LangGraph, AutoGen

Isolated integration tests verified in [`python/tests/test_v02_productization.py`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/python/tests/test_v02_productization.py):

| Framework | Tool Wrapper API | Fault Injection Pass | Zero False-Block Rate | Trace Correlation |
| :--- | :--- | :---: | :---: | :---: |
| **OpenAI Agents SDK** | `wrap_function(tool)` | 100% Pass | 100% Nominal Pass | Verified trace emission |
| **LangGraph** | `LangGraphToolWrapper` | 100% Pass | 100% Nominal Pass | Node state preserved |
| **AutoGen** | `AutoGenMiddleware` | 100% Pass | 100% Nominal Pass | Conversational context intact |
| **Microsoft Agent Framework** | `MicrosoftAgentMiddleware`| 100% Pass | 100% Nominal Pass | Execution envelope preserved |

---

## E. Pre-Inference Routing vs Post-Proposal Middleware (Composition)

| Layer | System | Primary Objective | Tokens / Turn | Execution Safety |
| :--- | :--- | :--- | :---: | :---: |
| **Pre-Inference** | AgentWeave / Top-K Router | Catalog candidate reduction | Reduced by ~60% | No fault recovery |
| **Post-Proposal** | Veyra Middleware | Contract validation & recovery | Unchanged | 0% DER, Safe Recovery |
| **Composed (C+D)** | Top-K + Veyra | Efficiency + Reliability | **Reduced by ~60%** | **0% DER, Safe Recovery** |

*Takeaway:* Veyra does not replace AgentWeave; the two systems operate at orthogonal layers (pre-inference prompt reduction vs post-proposal execution safety).

---

## F. Safety & Overhead Ledger

- **False Block Rate:** $0.0\%$ (100% pass on nominal control runs across 240 scenarios).
- **Wall-Clock Middleware Overhead:**
  - In-process Python callable: $18\text{--}35\,\mu\text{s}$.
  - Stdio MCP tool execution: $0.15\text{--}0.28\,\text{ms}$.
  - HTTP / Network tool execution: $< 0.5\,\text{ms}$ ($< 0.2\%$ relative overhead).
- **Token Overhead:** $0$ extra prompt tokens consumed on recovered faults (recovers deterministically without LLM replan).

---

## G. Product & Developer Ergonomics

```python
from veyra import Veyra

veyra = Veyra()
safe_tool = veyra.wrap(my_tool)
```
- Integration path: under 5 minutes from fresh checkout (`test_fresh_install_five_minute.py`).
- CLI tooling: `veyra doctor`, `veyra config explain`, `veyra inspect`, `veyra replay` operational.

---

## Final Anti-Drift Verdict & Promotion Decision

1. **Promoted into Production Default:**
   - Schema type coercion + Contract invariant enforcement.
   - Idempotency key stores & state-aware verify probes.
   - Safe abstention (`DEFER`/`DENY`) on ambiguous faults.
2. **Quarantined / Excluded from Production:**
   - Case Memory (experimental plugin only; no held-out transfer lift).
   - RL / LinUCB Bandits (lab exploration only).
   - LLM-as-a-Router (adds latency and non-determinism without safety benefit).
   - API Gateway / Proxy features (middleware remains strictly in-process).
