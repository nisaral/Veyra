# Veyra Claims & Evidence Ledger

**Audit Date:** October 7, 2026  
**Veyra Commit:** `HEAD` (Local Repository)  
**Rule:** Every numerical claim anywhere in the repository must map strictly to audited run metadata.

---

## 1. Verified Local Benchmark Evidence

| Metric / Claim | Benchmark & Version | Run Directory / File | Commit / Hash | Model & Agent | Seed(s) | Date | Metric Definition & Provenance |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **100.0% IPR** (+20.0pp vs static) | ContinuityBench-v1.0 | `benchmarks/continuitybench/scorecard_results.json` | `HEAD` (`c9a8f21`) | Deterministic Harness Agent | 2026 | 2026-10-07 | Intent Preservation Rate on N=40 held-out tasks |
| **100.0% IPR** (+84.0pp vs static) | ContinuityBench-300 | `benchmarks/continuitybench/results_300_scorecard.json` | `HEAD` (`c9a8f21`) | Deterministic Harness Agent | 2026 | 2026-10-07 | Intent Preservation Rate on N=100 hard tasks |
| **0 Duplicate Writes** (0% DER) | UndoBench-v1.0.1 (Local Verified) | `benchmarks/results/undobench_dev_results.json` | `HEAD` (`c9a8f21`) | Simulated / ReAct Driver | 42, 100, 2026 | 2026-10-07 | Duplicate Effect Rate under UNKNOWN_ACK lost response fault |
| **100.0% Recovery Success** | UndoBench-v1.0.1 (Local Verified) | `benchmarks/results/undobench_dev_results.json` | `HEAD` (`c9a8f21`) | Veyra-ZP / Veyra-Contract | 42, 100, 2026 | 2026-10-07 | Verification & recovery success under fault injection |
| **80.0% Recall@1, 0% Policy Violations** | Veyra Internal Resolution | `benchmarks/results/resolution_results.json` | `HEAD` (`c9a8f21`) | Deterministic Harness Agent | 42 | 2026-10-07 | Recall@1 & hard constraint policy violation rate across 240 scenarios |
| **0% False Block Rate** (100% Control Pass) | No-Op / Over-Blocking Harness | `benchmarks/results/over_blocking_report.json` | `HEAD` (`c9a8f21`) | ReAct Driver / Harness | 42 | 2026-10-07 | Rate of blocking or altering valid nominal tool calls in control runs |
| **$24.5\,\mu\text{s}$ p50 Resolution Latency** | Scalability & Resolution Harness | `benchmarks/results/resolution_results.json` | `HEAD` (`c9a8f21`) | Standalone Resolver | 42 | 2026-10-07 | Wall-clock resolution latency in microseconds |

---

## 2. External Benchmark Adapter Status Ledger

| External Benchmark | Version | Local Status | Implementation Provenance | Task Set & Env Notes |
| :--- | :--- | :---: | :--- | :--- |
| **UndoBench** | `v1.0.1` | `REPRODUCED_LOCAL` | `benchmarks/adapters/undobench/` | Pinned local runner, 50 DEV trials, 3 seeds |
| **ToolMisuseBench** | `v0.1` | `REPRODUCED_LOCAL` | `benchmarks/adapters/toolmisusebench/` | Pinned adapter, 5 fault categories |
| **MCPMark Verified** | `v1.2.0-verified` | `ADAPTER_READY` | `benchmarks/adapters/mcpmark/` | Docker container mock server runner ready |
| **MCP-Atlas** | `v1.0.0` | `ADAPTER_READY` | `benchmarks/adapters/mcp_atlas/` | API credential runner ready |
| **ComplexMCP** | `v1.0.0` | `ADAPTER_READY` | `benchmarks/adapters/complexmcp/` | 1,000 tool catalog scale runner ready |
| **ToolSandbox** | `v1.1.0` | `ADAPTER_READY` | `benchmarks/adapters/toolsandbox/` | Local sandbox environment adapter ready |
| **tau2-bench** | `v2.1.0` | `ADAPTER_READY` | `benchmarks/adapters/tau2/` | Maintained policy compliance runner ready |

---

## 3. Explicit Prohibited & Corrected Statements

- `ADAPTER_READY` indicates that the benchmark adapter implementation exists, but the benchmark suite has **not** been evaluated on live external services.
- `REPRODUCED_LOCAL` indicates that the actual benchmark runner was executed locally.
- Scripted/simulated drivers are recorded as unit/integration test runs, distinct from live frontier LLM benchmark evaluations.
- No competitor numbers copied from external papers or READMEs are claimed as Veyra benchmark results; unreproduced external architectures are labeled strictly as `RELATED SYSTEM`.
