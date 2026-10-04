"""Public benchmarks Veyra is aiming at, and the shifts in Veyra-ShiftBench.

This module is the checklist. It does not download or score the upstream
suites. A target stays ``not_run`` until a recorded command produces a table.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Target:
    id: str
    role: str
    status: str
    upstream: str


TARGETS: tuple[Target, ...] = (
    Target("mcpagentbench", "main external benchmark", "not_run",
           "official Brunestuder/MCPAgentBench; veyra mcpagentbench --agent react"),
    Target("bfcl-v4", "tool-selection baseline", "not_run",
           "Berkeley function-calling leaderboard, multi-turn and agentic splits"),
    Target("tau-bench", "stateful tool-agent-user", "not_run",
           "airline, retail, telecom, banking; use the current tau3 release"),
    Target("toolsandbox", "adaptation and recovery", "not_run",
           "stateful tools, implicit dependencies, intermediate checks"),
    Target("veyra-shiftbench", "benchmark this project contributes", "specified",
           "online tool shifts under a budget and a risk limit"),
    Target("gaia", "later broad-agent test", "not_scheduled",
           "general assistant questions; not the first score"),
    Target("swe-bench", "later generality check", "not_scheduled",
           "via the mini-swe-agent adapter, after the tool-routing claim"),
)

SHIFTS: tuple[str, ...] = (
    "unavailable",
    "expensive",
    "unreliable",
    "schema",
    "permission",
    "new_tool",
    "misleading",
)

ARMS: tuple[str, ...] = (
    "fixed-react",
    "static-router",
    "veyra-heuristic",
    "veyra-decision",
    "veyra-decision-bandit",
)

METRICS: tuple[str, ...] = (
    "task_success",
    "tool_selection_accuracy",
    "wasted_tool_calls",
    "cost_usd",
    "latency_ms",
    "recovery_after_shift",
    "unsafe_actions",
    "abstentions",
)


def checklist() -> str:
    lines = ["benchmarks (none of these is a measured Veyra score)", ""]
    for target in TARGETS:
        lines.append(f"  {target.status:14} {target.id:20} {target.role}")
    lines.append("")
    lines.append("arms: " + ", ".join(ARMS))
    lines.append("metrics: " + ", ".join(METRICS))
    lines.append("shiftbench: " + ", ".join(SHIFTS))
    return "\n".join(lines)
