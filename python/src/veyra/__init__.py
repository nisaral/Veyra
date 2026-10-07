"""Veyra: Vendor-Neutral Tool-Boundary Reliability and Resolution Layer for AI Agents.

Core Thesis:
Agent proposes -> Veyra resolves -> Tool executes.

Veyra provides execution-resolution, safe schema coercion, parameter alias remapping,
and declared fallback continuity when the agent's proposed tool path fails.
Deterministic by default with ZERO LLM dependency.
"""

from veyra.boundary import (
    FailureClassification,
    FailureKind,
    FailureProvenance,
    MCPToolMiddleware,
    SafeRetryPolicy,
    Veyra,
    VeyraBoundaryError,
)
from veyra.config import CapabilityConfig, EquivalenceConfig, load_equivalence_config
from veyra.core import (
    Decision,
    DecisionKind,
    ExecutableAction,
    ExecutionContract,
    ResolutionContract,
    ActionContract,
    ExecutionRequirements,
    ExecutionState,
    ExecutionTrace,
    RecoveryDecision,
    RecoveryDecisionKind,
    ReplanEvent,
    SideEffectClass,
    TraceSink,
    TrajectoryRecord,
)
from veyra.execution import ExecutionEngine
from veyra.export import StructuredTraceExporter
from veyra.plugins import PolicyPluginRegistry, default_plugins
from veyra.policy import (
    AdaptiveHistoryRoutePolicy,
    CalibratedUncertaintyEstimator,
    DeterministicRoutePolicy,
    OnlineExecutionMemory,
    RecoveryPolicy,
    ReliabilityAwareRoutePolicy,
    RiskLevel,
    RoutePolicy,
    SafeRecoveryPolicy,
    SelectiveResolutionPolicy,
    ToolReliabilityTracker,
    classify_risk,
)
from veyra.registry import (
    CandidateResolver,
    DeterministicCandidateResolver,
    ToolDefinition,
    ToolRegistry,
)

__version__ = "1.0.0"

__all__ = [
    # Top-level boundary middleware
    "Veyra",
    "MCPToolMiddleware",
    "VeyraBoundaryError",
    # Core abstractions
    "ExecutableAction",
    "ExecutionContract",
    "ResolutionContract",
    "ActionContract",
    "ExecutionRequirements",
    "SideEffectClass",
    "ExecutionState",
    "Decision",
    "DecisionKind",
    "RecoveryDecision",
    "RecoveryDecisionKind",
    "ExecutionTrace",
    "ReplanEvent",
    "TrajectoryRecord",
    "TraceSink",
    # Failure taxonomy & provenance
    "FailureKind",
    "FailureClassification",
    "FailureProvenance",
    # Tool Registry & Candidate Resolution
    "ToolRegistry",
    "ToolDefinition",
    "CandidateResolver",
    "DeterministicCandidateResolver",
    # Execution Engine
    "ExecutionEngine",
    # Routing & Recovery Policies
    "RoutePolicy",
    "DeterministicRoutePolicy",
    "RecoveryPolicy",
    "SafeRecoveryPolicy",
    "SafeRetryPolicy",
    "OnlineExecutionMemory",
    "AdaptiveHistoryRoutePolicy",
    "ToolReliabilityTracker",
    "ReliabilityAwareRoutePolicy",
    "CalibratedUncertaintyEstimator",
    "SelectiveResolutionPolicy",
    # Declarative Configuration
    "EquivalenceConfig",
    "CapabilityConfig",
    "load_equivalence_config",
    # Structured Trace Export
    "StructuredTraceExporter",
    # Plugins
    "PolicyPluginRegistry",
    "default_plugins",
    # Risk Classification
    "RiskLevel",
    "classify_risk",
]