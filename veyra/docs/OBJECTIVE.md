# Objective

Veyra is an open-source adaptive agent harness for cost- and risk-aware tool execution. It chooses the next action — tool, model, verification, retry, or abstention — under cost, latency, permission, and reliability constraints. A decision model returns probabilities. A policy is the only thing allowed to act.

The applied problem: agent workflows waste calls, pick a bad tool, retry it, and keep acting when they should abstain or switch. The measurable goal is higher task success per dollar, with fewer wasted and unsafe actions.

This is an applied systems project. The paper-shaped tracks in the rest of this repository are not the product.

## What a result has to show

On a fixed model and a fixed tool catalog, compare:

| arm | what it is |
|---|---|
| fixed ReAct | one loop, no router |
| static tool router | one tool order, no adaptation |
| Veyra heuristic | hand-written policy over the legal candidates |
| Veyra decision model | a local Jev-style model (Kev or an equivalent open model) scoring those candidates |
| Veyra decision model + bandit | the same scores, updated from logged outcomes |

Report task success, tool-selection accuracy, wasted tool calls, cost, latency, recovery after a shift, unsafe actions, and abstentions. Do not publish a headline until that table exists. The scripted 30-task suite under `out/` is a plumbing check. It is not this result.

## Proof, in order

1. **MCPAgentBench** is the main external benchmark. It already scores candidate-tool discrimination, multi-step tool use, completion, and execution efficiency.
2. **BFCL v4** is the tool-selection baseline (multi-turn and agentic categories).
3. **τ-bench (τ³ where that is the current release)** is the stateful user-and-tool benchmark (airline, retail, telecom, banking).
4. **ToolSandbox** is the adaptation and recovery benchmark: stateful tools, implicit dependencies, intermediate checks.
5. **Veyra-ShiftBench** is the benchmark this project contributes. The environment changes during the episode: a tool disappears, gets expensive, becomes unreliable, changes schema, loses permission, a new tool appears, or a tool returns a misleading result. The metric is how quickly and cheaply the agent adapts. MCPEvol-Bench studies schema evolution. ShiftBench studies online adaptation under a budget and a risk limit.
6. **GAIA**, then a **SWE-bench** adapter, come after the tool-routing claim is measured. They show the harness is not limited to one tool catalog. They are not the first score.

The README headline, once measured, is success, cost, and wasted calls against a fixed ReAct baseline on MCPAgentBench. Until then the README names the objective and does not invent the number.

## Not this milestone

A download site and a product dashboard are planned and not started. The trace viewer (`veyra view`) is the local inspection tool until a public result exists. The page, when it exists, should show the benchmark table, the decision trace, and how to install the harness. It should not be built ahead of the Kev arm on MCPAgentBench.
