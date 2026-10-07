"""Static Resolution Baseline (Phase 12 Specification).

Fair, rigorous competitor for Veyra:
- Receives EXACTLY the same:
  1. Equivalence declarations
  2. Argument alias maps
  3. Fallback candidate chains
  4. Hard safety constraints (idempotency, permissions, schema validation)
- Uses a simple deterministic strategy:
  1. First declared candidate in priority order
  2. Sequential fallback to next candidate if primary fails
- Crucially LACKS Veyra's distinctive capabilities:
  1. Dynamic ExecutionContract validation (freshness bounds, consistency, state assertions)
  2. Multi-history TAGE adaptive memory
  3. Real-time Beta-Bernoulli / CUSUM tool health tracking
  4. Calibrated selective resolution (SELECT / DEFER / DENY)
"""

from __future__ import annotations

import time
from typing import Any, Callable

from veyra.baseline.competent import coerce_argument_types, is_idempotent
from veyra.boundary.taxonomy import (
    FailureClassification,
    FailureKind,
    FailureProvenance,
    VeyraBoundaryError,
    classify_exception,
)
from veyra.core.action import ExecutableAction
from veyra.core.state import ExecutionState
from veyra.registry.tool_registry import ToolRegistry


class StaticResolutionMiddleware:
    """Static resolution middleware using fixed candidate priority and first-match fallback."""

    def __init__(self, registry: ToolRegistry):
        self.registry = registry

    def call(
        self,
        tool_name: str,
        arguments: dict[str, Any],
        idempotent: bool | None = None,
        state: ExecutionState | None = None,
    ) -> Any:
        exec_state = state or ExecutionState()

        # Build candidate chain: primary tool followed by declared equivalents and fallbacks
        candidate_names = [tool_name]
        equivs = self.registry.get_equivalents(tool_name)
        for eq in equivs:
            if eq not in candidate_names:
                candidate_names.append(eq)

        fallbacks = self.registry.get_fallback_chain(tool_name)
        for fb in fallbacks:
            if fb not in candidate_names:
                candidate_names.append(fb)

        # Hard constraint: filter by policy-allowed tools if present in state context
        allowed = exec_state.context.get("allowed_tools")
        if allowed is not None:
            candidate_names = [c for c in candidate_names if c in allowed]

        if not candidate_names:
            clf = FailureClassification(
                kind=FailureKind.AUTHORIZATION_ERROR,
                provenance=FailureProvenance.AUTHORIZATION_ERROR,
                retryable=False,
                repairable=False,
                requires_agent=True,
                safe_to_retry=False,
                message=f"No permitted candidates for '{tool_name}'",
            )
            raise VeyraBoundaryError(clf)

        last_error: Exception | None = None

        # Static resolution: iterate candidates in fixed priority order
        for cand_name in candidate_names:
            tool_def = self.registry.get(cand_name)
            if tool_def is None or tool_def.executable is None:
                continue

            # Hard constraint: permissions
            if tool_def.permissions and not set(tool_def.permissions).issubset(exec_state.permissions):
                continue

            # Argument alias remapping
            alias_map = self.registry.get_parameter_aliases(cand_name)
            remapped_args = dict(arguments)
            for prop_arg, canon_arg in alias_map.items():
                if prop_arg in remapped_args and canon_arg not in remapped_args:
                    remapped_args[canon_arg] = remapped_args.pop(prop_arg)

            # Schema coercion
            schema = tool_def.schema
            coerced_args = coerce_argument_types(remapped_args, schema)

            # Check required parameters
            if schema:
                required = schema.get("required") or schema.get("parameters", {}).get("required", [])
                if any(req not in coerced_args for req in required):
                    continue

            # Execute candidate
            try:
                result = tool_def.executable(**coerced_args)
                return result
            except Exception as exc:
                last_error = exc
                # If non-idempotent mutation failed, strict idempotency forbids blind fallback unless declared safe
                cand_idempotent = tool_def.idempotent if idempotent is None else idempotent
                if not cand_idempotent:
                    # Halt immediately on failed mutation
                    clf = classify_exception(exc, safe=False)
                    raise VeyraBoundaryError(clf, original_exc=exc) from exc

        # All candidates exhausted or failed
        if last_error is not None:
            clf = classify_exception(last_error, safe=True)
            raise VeyraBoundaryError(clf, original_exc=last_error) from last_error

        clf = FailureClassification(
            kind=FailureKind.PRECONDITION_ERROR,
            provenance=FailureProvenance.PRECONDITION_ERROR,
            retryable=False,
            repairable=False,
            requires_agent=True,
            safe_to_retry=False,
            message="All declared static resolution candidates failed or invalid",
        )
        raise VeyraBoundaryError(clf)
