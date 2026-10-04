"""Run official MCPAgentBench without changing its tasks or evaluator.

The checkout lives in ``third_party/MCPAgentBench``. ReAct mode calls their
``run_experiment`` as-is. Veyra modes wrap each MCP tool so the same LLM still
proposes the call, then a policy may drop a wasted retry before the tool runs.

A result is only a public score when a real model client is configured in
their ``configs/llm_config.json`` and the run finishes under that client.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

def bench_root() -> Path:
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        candidate = parent / "third_party" / "MCPAgentBench"
        if (candidate / "runbenchmark.py").is_file():
            return candidate
    raise FileNotFoundError(
        "MCPAgentBench is missing. From the repo root: "
        "git submodule update --init third_party/MCPAgentBench"
    )


def require_api_key(env_name: str = "ROUTER_API_KEY") -> str:
    key = os.environ.get(env_name) or os.environ.get("OPENROUTER_API_KEY") or os.environ.get("OPENAI_API_KEY")
    if not key:
        raise RuntimeError(
            "No LLM API key. MCPAgentBench's official client reads "
            "ROUTER_API_KEY (OpenRouter) from the environment. Set that, then "
            "run: veyra mcpagentbench --agent react --model openai/gpt-4o-mini"
        )
    return key


async def run_official(*, model: str, tasks_type: str, concurrency: int | None,
                       num_servers: int | None, output_name: str | None) -> float:
    """Vanilla ReAct: the benchmark's own AutoGen agent, unchanged."""
    root = bench_root()
    cwd = os.getcwd()
    os.chdir(root)
    sys.path.insert(0, str(root))
    try:
        from src.experiment import run_experiment

        return await run_experiment(
            model=model,
            tasks_type=tasks_type,
            concurrency=concurrency,
            num_servers=num_servers,
            output_name=output_name,
        )
    finally:
        os.chdir(cwd)


def wrap_tools_with_policy(tools: list, policy: str) -> list:
    """Gate MCP tools. The LLM still proposes the call; the policy may refuse it.

    ``react`` leaves the list untouched. ``heuristic`` drops a tool after two
    consecutive failures of the same name. Gold labels are never consulted.
    """
    if policy in {"react", "none", ""}:
        return tools
    fails: dict[str, int] = {}

    wrapped = []
    for tool in tools:
        wrapped.append(_gate_one(tool, fails, policy))
    return wrapped


def _gate_one(tool, fails: dict[str, int], policy: str):
    name = getattr(tool, "name", None) or (tool.schema.get("name") if hasattr(tool, "schema") else "tool")
    original = tool.run if hasattr(tool, "run") else tool

    async def run(*args, **kwargs):
        if policy == "heuristic" and fails.get(name, 0) >= 2:
            return f"veyra policy dropped {name}: two consecutive failures"
        try:
            out = original(*args, **kwargs)
            if hasattr(out, "__await__"):
                out = await out
            fails[name] = 0
            return out
        except Exception:
            fails[name] = fails.get(name, 0) + 1
            raise

    if hasattr(tool, "run"):
        tool.run = run  # type: ignore[method-assign]
        return tool
    return run


async def run_veyra(*, model: str, tasks_type: str, policy: str,
                    concurrency: int | None, num_servers: int | None,
                    output_name: str | None) -> float:
    """Same experiment, tools gated by a Veyra policy. Evaluator is theirs."""
    root = bench_root()
    cwd = os.getcwd()
    os.chdir(root)
    sys.path.insert(0, str(root))
    try:
        import src.agenttest as agenttest
        from src.experiment import run_experiment

        original = agenttest.construct_agent

        async def construct_agent(client, task_correct_tools, num_servers, num_tools):
            assistant = await original(client, task_correct_tools, num_servers, num_tools)
            tools = getattr(assistant, "_tools", None) or getattr(assistant, "tools", None)
            if tools:
                gated = wrap_tools_with_policy(list(tools), policy)
                if hasattr(assistant, "_tools"):
                    assistant._tools = gated
                elif hasattr(assistant, "tools"):
                    assistant.tools = gated
            return assistant

        agenttest.construct_agent = construct_agent
        try:
            return await run_experiment(
                model=model,
                tasks_type=tasks_type,
                concurrency=concurrency,
                num_servers=num_servers,
                output_name=output_name,
            )
        finally:
            agenttest.construct_agent = original
    finally:
        os.chdir(cwd)
