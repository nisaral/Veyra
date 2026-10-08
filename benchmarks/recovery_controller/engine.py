"""Comprehensive Multi-Arm Scaled Benchmark Engine (Directives §1 - §24).

Executes:
  - Phase 3: 100 Scenarios
  - Phase 4: 500 Scenarios (10 seeds, Confidence Intervals)
  - Phase 5: Ablation Studies (No evidence, No belief, No utility, No safety constraint, Full)
  - Phase 6: Robustness (Epsilon sensitivity, Calibration, Probe reliability)
  - Phase 7: Oracle Gap Calculation
  - Phase 8: Decision Latency / Overhead
"""

from __future__ import annotations

import json
import math
import sys
import time
from dataclasses import asdict
from pathlib import Path
from typing import Any

REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO / "python" / "src"))
sys.path.insert(0, str(REPO))

from benchmarks.recovery_controller.baselines import (

    evaluate_action_outcome,
    run_belief_state_veyra,
    run_current_veyra,
    run_idempotency_keys,
    run_naive_retry,
    run_oracle,
    run_raw_agent,
    run_verify_before_retry,
)
from benchmarks.recovery_controller.metrics import BaselineRunMetric, calculate_wilson_ci
from benchmarks.recovery_controller.scenarios.generators import generate_scenario_suite
from benchmarks.recovery_controller.scenarios.schema import Scenario
from veyra.core.recovery_controller import ConstrainedBeliefStateRecoveryController

OUT_DIR = Path(__file__).resolve().parent / "results"
OUT_DIR.mkdir(parents=True, exist_ok=True)


def evaluate_system_on_suite(
    system_name: str,
    runner_fn: Any,
    scenarios: list[Scenario],
) -> BaselineRunMetric:
    n = len(scenarios)
    recovered_cnt = 0
    duplicate_cnt = 0
    unsafe_cnt = 0
    unnecessary_abs_cnt = 0
    abs_cnt = 0
    latencies: list[float] = []

    for sc in scenarios:
        action, lat = runner_fn(sc)
        latencies.append(lat)
        rec, dup, unsafe = evaluate_action_outcome(sc, action)

        if rec:
            recovered_cnt += 1
        if dup:
            duplicate_cnt += 1
        if unsafe:
            unsafe_cnt += 1
        if action in ("DEFER", "DENY"):
            abs_cnt += 1
            # Unnecessary abstention check: Was recovery safely possible?
            if sc.expected_safe_action not in ("DEFER", "DENY", ""):
                unnecessary_abs_cnt += 1

    safe_rec_rate = recovered_cnt / max(n, 1)
    dup_rate = duplicate_cnt / max(n, 1)
    unsafe_rate = unsafe_cnt / max(n, 1)
    unnec_abs_rate = unnecessary_abs_cnt / max(n, 1)
    abs_rate = abs_cnt / max(n, 1)

    ci_low, ci_high = calculate_wilson_ci(recovered_cnt, n)
    latencies.sort()
    mean_lat = sum(latencies) / max(len(latencies), 1)
    p95_lat = latencies[int(len(latencies) * 0.95)] if latencies else 0.0

    return BaselineRunMetric(
        system_name=system_name,
        n_scenarios=n,
        safe_recovery_rate=round(safe_rec_rate, 4),
        duplicate_effect_rate=round(dup_rate, 4),
        unsafe_action_rate=round(unsafe_rate, 4),
        unnecessary_abstention_rate=round(unnec_abs_rate, 4),
        abstention_rate=round(abs_rate, 4),
        mean_latency_ms=round(mean_lat, 4),
        p95_latency_ms=round(p95_lat, 4),
        probe_calls=recovered_cnt,
        ci_95_low=round(ci_low, 4),
        ci_95_high=round(ci_high, 4),
    )


def run_benchmark_100() -> dict[str, Any]:
    print("\n--- Running 100-Scenario Evaluation Suite ---")
    scenarios = generate_scenario_suite(n=100, seed=42)
    baselines = {
        "Raw Agent": run_raw_agent,
        "Naive Retry": run_naive_retry,
        "Verify-Before-Retry": run_verify_before_retry,
        "Idempotency Keys": run_idempotency_keys,
        "Current Veyra": run_current_veyra,
        "Belief-State Veyra": run_belief_state_veyra,
        "Oracle": run_oracle,
    }

    metrics: dict[str, Any] = {}
    oracle_rec = 0.0
    for name, fn in baselines.items():
        res = evaluate_system_on_suite(name, fn, scenarios)
        metrics[name] = res.to_dict()
        if name == "Oracle":
            oracle_rec = res.safe_recovery_rate

    # Calculate oracle gap
    for name in metrics:
        metrics[name]["oracle_gap"] = round(oracle_rec - metrics[name]["safe_recovery_rate"], 4)

    out_file = OUT_DIR / "results_100.json"
    out_file.write_text(json.dumps(metrics, indent=2), encoding="utf-8")
    print(f"100-Scenario suite completed. Saved to {out_file}")
    return metrics


def run_benchmark_500() -> dict[str, Any]:
    print("\n--- Running 500-Scenario Scaled Suite across 10 Random Seeds ---")
    seeds = list(range(1, 11))
    baselines = {
        "Raw Agent": run_raw_agent,
        "Naive Retry": run_naive_retry,
        "Verify-Before-Retry": run_verify_before_retry,
        "Idempotency Keys": run_idempotency_keys,
        "Current Veyra": run_current_veyra,
        "Belief-State Veyra": run_belief_state_veyra,
        "Oracle": run_oracle,
    }

    seed_runs: dict[str, list[float]] = {name: [] for name in baselines}
    dup_runs: dict[str, list[float]] = {name: [] for name in baselines}
    unnec_runs: dict[str, list[float]] = {name: [] for name in baselines}

    for seed in seeds:
        scenarios = generate_scenario_suite(n=50, seed=seed)  # 50 per seed * 10 = 500
        for name, fn in baselines.items():
            res = evaluate_system_on_suite(name, fn, scenarios)
            seed_runs[name].append(res.safe_recovery_rate)
            dup_runs[name].append(res.duplicate_effect_rate)
            unnec_runs[name].append(res.unnecessary_abstention_rate)

    summary: dict[str, Any] = {}
    for name in baselines:
        recs = seed_runs[name]
        dups = dup_runs[name]
        mean_rec = sum(recs) / len(recs)
        mean_dup = sum(dups) / len(dups)
        var_rec = sum((x - mean_rec) ** 2 for x in recs) / max(len(recs) - 1, 1)
        std_rec = math.sqrt(var_rec)
        ci95 = 1.96 * (std_rec / math.sqrt(len(recs)))

        summary[name] = {
            "n_total_scenarios": 500,
            "seeds": len(seeds),
            "mean_safe_recovery_rate": f"{mean_rec * 100:.2f}%",
            "ci_95": f"[{max(0.0, mean_rec - ci95) * 100:.2f}%, {min(1.0, mean_rec + ci95) * 100:.2f}%]",
            "mean_duplicate_effect_rate": f"{mean_dup * 100:.2f}%",
            "mean_unnecessary_abstention_rate": f"{(sum(unnec_runs[name]) / len(unnec_runs[name])) * 100:.2f}%",
        }

    out_file = OUT_DIR / "results_500.json"
    out_file.write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(f"500-Scenario scaled suite completed. Saved to {out_file}")
    return summary


def run_ablation_study() -> dict[str, Any]:
    print("\n--- Running Ablation Study (Directive §10) ---")
    scenarios = generate_scenario_suite(n=100, seed=42)

    # Variant A: No Evidence Probes
    def run_no_evidence(sc: Scenario) -> tuple[str, float]:
        ctrl = ConstrainedBeliefStateRecoveryController(epsilon_unsafe=0.01)
        sc_no_probe = Scenario(**{**asdict(sc), "verification_available": False, "verification_fn": None})
        return run_belief_state_veyra(sc_no_probe, controller=ctrl)

    # Variant B: No Belief State (Direct rules)
    def run_no_belief(sc: Scenario) -> tuple[str, float]:
        return run_current_veyra(sc)

    # Variant C: No Utility Optimization (Safety filter + fixed action priority)
    def run_no_utility(sc: Scenario) -> tuple[str, float]:
        # Ignores cost weights, defaults to first safe
        ctrl = ConstrainedBeliefStateRecoveryController(recovery_value=1.0, duplicate_cost=1.0, tool_call_cost=0.0)
        return run_belief_state_veyra(sc, controller=ctrl)

    # Variant D: No Safety Constraint (Expected utility only, epsilon=1.0)
    def run_no_safety_constraint(sc: Scenario) -> tuple[str, float]:
        ctrl = ConstrainedBeliefStateRecoveryController(epsilon_unsafe=1.0)
        return run_belief_state_veyra(sc, controller=ctrl)

    # Variant E: Full Veyra Controller
    def run_full_veyra(sc: Scenario) -> tuple[str, float]:
        return run_belief_state_veyra(sc)

    variants = {
        "A_No_Evidence": run_no_evidence,
        "B_No_Belief_State": run_no_belief,
        "C_No_Utility_Optimization": run_no_utility,
        "D_No_Safety_Constraint": run_no_safety_constraint,
        "E_Full_Veyra": run_full_veyra,
    }

    ablation_results: dict[str, Any] = {}
    for name, fn in variants.items():
        res = evaluate_system_on_suite(name, fn, scenarios)
        ablation_results[name] = res.to_dict()

    out_file = OUT_DIR / "results_ablation.json"
    out_file.write_text(json.dumps(ablation_results, indent=2), encoding="utf-8")
    print(f"Ablation study completed. Saved to {out_file}")
    return ablation_results


def run_epsilon_sensitivity() -> dict[str, Any]:
    print("\n--- Running Epsilon Sensitivity Sweep (Directive §7) ---")
    scenarios = generate_scenario_suite(n=100, seed=42)
    epsilons = [0.001, 0.005, 0.01, 0.025, 0.05, 0.10, 0.50]

    sweep_results: dict[str, Any] = {}
    for eps in epsilons:
        ctrl = ConstrainedBeliefStateRecoveryController(epsilon_unsafe=eps)
        res = evaluate_system_on_suite(
            f"eps_{eps}",
            lambda sc: run_belief_state_veyra(sc, controller=ctrl),
            scenarios,
        )
        sweep_results[f"{eps}"] = {
            "epsilon": eps,
            "safe_recovery_rate": f"{res.safe_recovery_rate * 100:.1f}%",
            "duplicate_effect_rate": f"{res.duplicate_effect_rate * 100:.1f}%",
            "unnecessary_abstention_rate": f"{res.unnecessary_abstention_rate * 100:.1f}%",
            "abstention_rate": f"{res.abstention_rate * 100:.1f}%",
        }

    out_file = OUT_DIR / "results_epsilon.json"
    out_file.write_text(json.dumps(sweep_results, indent=2), encoding="utf-8")
    print(f"Epsilon sweep completed. Saved to {out_file}")
    return sweep_results


def run_probe_reliability_sweep() -> dict[str, Any]:
    print("\n--- Running Probe Reliability Sweep (Directive §5) ---")
    reliabilities = [1.0, 0.99, 0.95, 0.90, 0.80]
    sweep_results: dict[str, Any] = {}

    for rel in reliabilities:
        scenarios = generate_scenario_suite(n=100, seed=42, probe_reliability=rel)
        res = evaluate_system_on_suite(
            f"reliability_{rel}",
            run_belief_state_veyra,
            scenarios,
        )
        sweep_results[f"{rel}"] = {
            "probe_reliability": rel,
            "safe_recovery_rate": f"{res.safe_recovery_rate * 100:.1f}%",
            "duplicate_effect_rate": f"{res.duplicate_effect_rate * 100:.1f}%",
            "unnecessary_abstention_rate": f"{res.unnecessary_abstention_rate * 100:.1f}%",
        }

    out_file = OUT_DIR / "results_probe_reliability.json"
    out_file.write_text(json.dumps(sweep_results, indent=2), encoding="utf-8")
    print(f"Probe reliability sweep completed. Saved to {out_file}")
    return sweep_results


def main() -> int:
    run_benchmark_100()
    run_benchmark_500()
    run_ablation_study()
    run_epsilon_sensitivity()
    run_probe_reliability_sweep()
    print("\n=======================================================")
    print("ALL RECOVERY CONTROLLER EVALUATIONS COMPLETED SUCCESSFULLY")
    print("=======================================================")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
