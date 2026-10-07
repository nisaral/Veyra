"""Veyra Plugins: RoutePolicy Plugin Registry.

Enables extending Veyra with custom routing strategies while keeping
core deterministic policies drop-in and free of external dependencies.
"""

from __future__ import annotations

from typing import Any, Callable

from veyra.policy.base import RoutePolicy
from veyra.policy.deterministic import DeterministicRoutePolicy
from veyra.policy.history_adaptive import AdaptiveHistoryRoutePolicy
from veyra.policy.learning import BanditRoutePolicy
from veyra.policy.reliability import ReliabilityAwareRoutePolicy
from veyra.policy.selective import SelectiveResolutionPolicy

PolicyFactory = Callable[..., RoutePolicy]


class PolicyPluginRegistry:
    """Central registry for built-in and third-party RoutePolicy plugins."""

    def __init__(self):
        self._factories: dict[str, PolicyFactory] = {}
        self._register_defaults()

    def _register_defaults(self) -> None:
        self.register("deterministic", lambda **kw: DeterministicRoutePolicy())
        self.register("tage", lambda **kw: AdaptiveHistoryRoutePolicy())
        self.register("reliability", lambda **kw: ReliabilityAwareRoutePolicy())
        self.register("selective", lambda **kw: SelectiveResolutionPolicy())
        self.register("bandit", lambda **kw: BanditRoutePolicy(**kw))

    def register(self, name: str, factory: PolicyFactory) -> None:
        """Register a new policy plugin factory."""
        self._factories[name.lower()] = factory

    def get(self, name: str, **kwargs: Any) -> RoutePolicy:
        """Instantiate a policy by plugin name."""
        key = name.lower()
        if key not in self._factories:
            raise KeyError(f"Unknown RoutePolicy plugin '{name}'. Available: {list(self._factories.keys())}")
        return self._factories[key](**kwargs)

    def list_plugins(self) -> list[str]:
        return sorted(self._factories.keys())


# Global default plugin registry
default_plugins = PolicyPluginRegistry()
