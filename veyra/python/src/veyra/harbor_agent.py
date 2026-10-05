"""Harbor custom agent: Veyra as an *external* loop.

Harbor loads this with::

    harbor run -d terminal-bench@2.0 -a veyra.harbor_agent:VeyraHarborAgent

The class subclasses Harbor's BaseAgent when ``harbor`` is installed. Without
Harbor, it is still importable so the rest of the package and tests stay green.

This agent does not reimplement Terminus-2. It execs through BaseEnvironment
and fails open (continues) if the controller raises. Shadow mode logs a
would-have-switched HandoffV1 without changing the environment.
"""

from __future__ import annotations

from typing import Any

try:
    from harbor.agents.base import BaseAgent  # type: ignore
except ImportError:  # Harbor is optional until a paid eval.
    class BaseAgent:  # type: ignore[no-redef]
        pass


class VeyraHarborAgent(BaseAgent):
    def __init__(self, logs_dir=None, model_name: str | None = None, shadow: bool = True, **kwargs: Any):
        self.logs_dir = logs_dir
        self.model_name = model_name
        self.shadow = shadow
        self.kwargs = kwargs

    @staticmethod
    def name() -> str:
        return "veyra"

    def version(self) -> str | None:
        from veyra import __version__

        return __version__

    async def setup(self, environment: Any) -> None:
        return None

    async def run(self, instruction: str, environment: Any, context: Any = None) -> None:
        """Fail-open: environment work happens even if the controller errors."""
        try:
            from veyra.handoff import shadow_record

            if self.shadow:
                shadow_record(instruction, reason="harbor-agent-shadow")
        except Exception:
            pass
        exec_fn = getattr(environment, "exec", None)
        if callable(exec_fn):
            result = exec_fn(instruction)
            if hasattr(result, "__await__"):
                await result
