"""Decision backends.

Every backend answers one question: given this state and these *legal*
candidates, what is the probability that each is the right next action?

Backends never invent candidates, never execute anything, and never widen the
action space. The kernel re-validates every choice regardless.
"""

from veyra.decision.base import DecisionModel, decide, probabilities
from veyra.decision.bandit import LinUCBBandit
from veyra.decision.heuristic import HeuristicDecision
from veyra.decision.oracle import OracleDecision
from veyra.decision.von import VonDecision

BACKENDS: dict[str, type[DecisionModel]] = {
    "heuristic": HeuristicDecision,
    "von": VonDecision,
    "bandit": LinUCBBandit,
    "oracle": OracleDecision,
}


def get_backend(name: str, **kwargs) -> DecisionModel:
    key = name.split(":", 1)[0]
    cls = BACKENDS.get(key)
    if cls is None:
        raise KeyError(f"unknown decision backend {name!r}; known: {sorted(BACKENDS)}")
    return cls(**kwargs)


__all__ = [
    "DecisionModel",
    "HeuristicDecision",
    "VonDecision",
    "LinUCBBandit",
    "OracleDecision",
    "BACKENDS",
    "get_backend",
    "decide",
    "probabilities",
]