# Objective

Veyra is a **switch controller, handoff compiler, and measurement kit** for existing agent harnesses. It is not a rival meta-runtime.

Databricks Omnigent already sits above Claude Code, Codex, and Pi. Veyra does the three things that layer does not:

1. **When to switch**, with calibrated risk, fail-open if the controller is uncertain.
2. **What state crosses** (`HandoffV1`: decisions, failed actions, open subgoals, verification).
3. **Whether the switch was worth it**: oracle gap with a same-harness null, rescue rate, cost at non-inferior success.

The decision model (Kev) **selects**. It does not write. An extractor proposes candidate items from the trace; Kev scores which to keep under a budget. The bar is embedding-based pruning, not a generative summarizer.

The kernel policy is the only component that may act. If the controller errors or times out, the current harness continues.

## What is not a result

- The bundled 30-task suite and `out/kev/` are **synthetic/scripted**.
- One MCPAgentBench Gemma smoke (`TFS 0.0` on 1 day task) is plumbing. Gemma did not emit native tool calls. MCPAgentBench is a **smoke test**, not the primary public claim.

## Gate 1 (pre-registered)

Run on Harbor, mid-strength model, at least one **external** harness (mini-swe-agent or Terminus-2) plus Veyra reference harnesses.

| Quantity | Definition |
|---|---|
| Cross-harness oracle | Best of *k* harnesses per task |
| Same-harness null | Best of *k* seeds of the **same** harness |
| Net headroom | Cross-harness oracle minus same-harness null |
| Cost axis | Mean $ at non-inferior success (5pp margin) |

**Stop rule:** stop claiming switch value if the 95% CI **upper bound** on net headroom is below 2pp **and** there is no cost win at non-inferior success.

**Rescue rate (handoff, not routing):** stall under H1, fork at a checkpoint to H2 vs fork back to H1 vs fresh restart in H1. Without the same-harness fork, “another attempt” explains the gain.

Pilot **20 tasks** to measure $/run before the full matrix.

## Releases

| Release | Contents | Gate |
|---|---|---|
| v0.3 | Adapters, HandoffV1, shadow mode, trace export | Offline suite green |
| v0.4 | Headroom report with nulls, outcome-matrix dataset | Gate 1 passed **or published null** |
| v0.5 | View-spectrum, cost-Pareto | Non-inferiority shown |
| v1.0 | Learned selector only if it beats embedding pruning | Earned |

## Production (do not defer)

Shadow mode, fail-open, irreversibility guard (snapshot before switch), versioned HandoffV1, token/$ caps, OpenTelemetry export, `make reproduce`.
