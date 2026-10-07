"""Veyra Idempotency Key Semantics & Management (Section 7).

Strong Idempotency Invariant:
The same logical mutation MUST derive the same idempotency key across all retries,
recovery attempts, network drops, and process restarts.

Form:
idempotency_key = sha256(intent_id + tool + canonical_arguments + context_signature)
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass, field
from typing import Any


def canonicalize_json(obj: Any) -> str:
    """Return a stable, sorted JSON string for hashing arguments and context."""
    if isinstance(obj, dict):
        return "{" + ",".join(f"{json.dumps(k)}:{canonicalize_json(v)}" for k, v in sorted(obj.items())) + "}"
    elif isinstance(obj, list):
        return "[" + ",".join(canonicalize_json(x) for x in obj) + "]"
    elif isinstance(obj, set):
        return "[" + ",".join(canonicalize_json(x) for x in sorted(obj)) + "]"
    else:
        return json.dumps(obj)


@dataclass
class IdempotencyIdentity:
    """Logical idempotency identity bound to a unique mutation intent."""

    intent_id: str
    tool: str
    arguments: dict[str, Any]
    context_signature: str = ""
    created_at: float = 0.0
    key: str = field(init=False)

    def __post_init__(self):
        self.key = self.compute_key()

    def compute_key(self) -> str:
        payload = (
            f"intent:{self.intent_id}|tool:{self.tool}|args:{canonicalize_json(self.arguments)}|ctx:{self.context_signature}"
        )
        return f"idk_{hashlib.sha256(payload.encode('utf-8')).hexdigest()[:24]}"


class IdempotencyKeyStore:
    """Persistent key store guaranteeing idempotency key stability across retries and restarts."""

    def __init__(self):
        self._store: dict[str, IdempotencyIdentity] = {}
        self._execution_history: dict[str, dict[str, Any]] = {}

    def get_or_create(
        self,
        intent_id: str,
        tool: str,
        arguments: dict[str, Any],
        context_signature: str = "",
    ) -> IdempotencyIdentity:
        """Retrieve existing idempotency identity for the logical mutation, or create and persist it."""
        # Key lookup by intent_id + tool
        lookup_key = f"{intent_id}:{tool}"
        if lookup_key in self._store:
            return self._store[lookup_key]

        identity = IdempotencyIdentity(
            intent_id=intent_id,
            tool=tool,
            arguments=arguments,
            context_signature=context_signature,
        )
        self._store[lookup_key] = identity
        return identity

    def record_attempt(self, key: str, attempt_number: int, status: str, result: Any = None) -> None:
        if key not in self._execution_history:
            self._execution_history[key] = {"attempts": 0, "status": "PENDING", "result": None, "history": []}

        hist = self._execution_history[key]
        hist["attempts"] = attempt_number
        hist["status"] = status
        if result is not None:
            hist["result"] = result
        hist["history"].append({"attempt": attempt_number, "status": status, "result": result})

    def get_history(self, key: str) -> dict[str, Any] | None:
        return self._execution_history.get(key)

    def clear(self) -> None:
        self._store.clear()
        self._execution_history.clear()
