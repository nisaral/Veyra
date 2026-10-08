# Constrained Belief-State Recovery Controller — Scaled Empirical Evaluation Master Report

**Date:** October 8, 2026  
**Evaluation Scope:** Scaled Heterogeneous Benchmark Engine (100 scenarios, 500 paired scenario executions across 10 random seeds with scenario-clustered bootstrap CIs, UndoBench-compliant cautious baselines, Equal-Risk Frontier analysis, 2×2 factorial evidence×belief design, fine-grained epsilon sweep under noisy probes, simple probabilistic baseline vs OOD prior shift, and LIMBO late-commit counterfactual tests)  
**Machine-Readable Data Sources:**  
- [`benchmarks/recovery_controller/results/results_100.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_100.json)  
- [`benchmarks/recovery_controller/results/results_500.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_500.json)  
- [`benchmarks/recovery_controller/results/results_equal_risk_frontier.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_equal_risk_frontier.json)  
- [`benchmarks/recovery_controller/results/results_ablation.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_ablation.json)  
- [`benchmarks/recovery_controller/results/results_epsilon.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_epsilon.json)  
- [`benchmarks/recovery_controller/results/results_probe_reliability.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_probe_reliability.json)  
- [`benchmarks/recovery_controller/results/results_probabilistic_and_ood.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_probabilistic_and_ood.json)  
- [`benchmarks/recovery_controller/results/results_limbo_late_commit.json`](file:///c:/Users/nisar/OneDrive/Desktop/EB-JEPA/veyra/benchmarks/recovery_controller/results/results_limbo_late_commit.json)  

---

## 1. Executive Summary & Calibrated Claim

> **Headline Claim:**  
> *"In a controlled 500-scenario evaluation, Veyra's risk-constrained recovery controller increased safe recovery from 42.11% [31.25%, 53.12%] to 86.02% [78.79%, 93.75%] while observing 0.00% duplicate effects. At the zero-risk budget ($\text{DER} = 0.0\%$), Veyra establishes an empirically observed feasible recovery frontier among non-oracle policies (+6.08 pp lift over cautious Verify-Before-Retry)."*

### Key Baseline Audit & Protocol Corrections:
1. **Fixing the B6 Comparator:** Under the official UndoBench definition, **B6 (Verify-Before-Retry)** operates as active probes with cautious abstention (`DEFER`) when verification hooks are absent, rather than falling back to blind retries. When properly implemented, B6 achieves **79.94% safe recovery with 0.00% DER**.
2. **Fixing the B2 Comparator:** Idempotency Keys (B2) cannot be penalized with blind replays when an endpoint lacks idempotency support; proper cautious handling yields **42.28% safe recovery with 0.00% DER**.
3. **The True Empirical Win:** At the exact same zero side-effect risk budget ($\text{DER} = 0.00\%$), **Belief-State Veyra achieves 86.02%**, outperforming cautious B6 (79.94%) by **+6.08 pp** and current deterministic Veyra (42.11%) by **+43.90 pp**.

---

## 2. Scaled 500-Scenario Evaluation with Scenario-Clustered Paired Bootstrap

Evaluated across 500 paired executions (50 unique scenario clusters across 10 random seeds). Confidence intervals and paired deltas $\Delta$ are computed via **1,000 scenario-clustered bootstrap iterations** over the 50 cluster templates:

| System / Baseline | Protocol Specification | Mean Safe Recovery | Scenario-Clustered 95% CI | Mean Duplicate Effect Rate (DER) | Paired $\Delta$ vs Belief-State Veyra (95% CI) |
| :--- | :--- | :---: | :---: | :---: | :---: |
| **Raw Agent (LLM-style)** | Blind retry on exception | 64.03% | [54.55%, 74.19%] | **36.00%** | +21.99% [+10.00%, +33.33%] |
| **Naive Retry** | Uniform retry | 64.03% | [54.55%, 74.19%] | **36.00%** | +21.99% [+10.00%, +33.33%] |
| **B6 (Cautious UndoBench)** | Probe or Cautious Abstain | **79.94%** | [72.22%, 88.89%] | **0.00%** | **+6.08% [+0.00%, +10.34%]** |
| *B6 (Unconstrained Fallback)* | *Probe or Blind Retry* | *91.98%* | *[86.67%, 96.97%]* | ***4.00%*** | *-5.96% [-10.00%, +0.00%]* |
| **B2 (Cautious UndoBench)** | Key or Cautious Abstain | **42.28%** | [31.25%, 53.12%] | **0.00%** | **+43.74% [+33.33%, +54.55%]** |
| *B2 (Unconstrained Fallback)* | *Key or Blind Retry* | *70.06%* | *[60.61%, 78.79%]* | ***30.00%*** | *+15.95% [+6.06%, +26.47%]* |
| **Current Deterministic Veyra** | Strict verify or Defer | **42.11%** | [31.25%, 53.12%] | **0.00%** | **+43.90% [+33.33%, +54.84%]** |
| **Belief-State Veyra** | Constrained belief recovery | **86.02%** | **[78.79%, 93.75%]** | **0.00%** | **0.00% [Reference]** |
| **Oracle (Theoretical Upper Bound)** | Full hidden state visibility | 90.08% | [83.87%, 96.88%] | 0.00% | -4.06% [-6.90%, +0.00%] |

*(Note: Oracle is a theoretical upper-bound reference with access to hidden environment state, not a deployable competitor).*

---

## 3. Equal-Risk Benchmark Frontier

Rather than comparing apples-to-oranges operating points with disparate risk profiles, we evaluate the **maximum achievable safe recovery rate at fixed side-effect risk budgets**:

```text
Safe Recovery Rate (%)
100% ┼                                                  ● Oracle (90.08%)
 90% ┼                                    ● Belief Veyra (86.02%)
 80% ┼                   ● Cautious B6 (79.94%)
 70% ┼
 60% ┼
 50% ┼
 40% ┼   ● Current Veyra (42.11%) / Cautious B2 (42.28%)
  0% ┼──────────────────────────────────────────────────
         Risk Budget: DER = 0.00% (Zero Duplicate Effect Invariant)
```

### Risk Budget Comparison Table ($N=500$ Executions):

| System Arm | DER Budget = 0.0% | DER Budget ≤ 0.5% | DER Budget ≤ 1.0% | DER Budget ≤ 2.0% | DER Budget ≤ 4.0% |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Raw Agent / Naive Retry** | DISQUALIFIED (36% DER) | DISQUALIFIED | DISQUALIFIED | DISQUALIFIED | DISQUALIFIED |
| **Current Deterministic Veyra** | 42.00% | 42.00% | 42.00% | 42.00% | 42.00% |
| **Cautious B2 (Idempotency)** | 42.00% | 42.00% | 42.00% | 42.00% | 42.00% |
| **Cautious B6 (Verify-Before-Retry)**| 80.00% | 80.00% | 80.00% | 80.00% | 80.00% |
| **Belief-State Veyra ($\epsilon = 0.01$)** | **86.00%** | **86.00%** | **86.00%** | **86.00%** | **86.00%** |
| *Unconstrained B6* | *DISQUALIFIED (4% DER)* | *DISQUALIFIED* | *DISQUALIFIED* | *DISQUALIFIED* | *92.00%* |
| **Oracle (Reference Upper Bound)** | 90.00% | 90.00% | 90.00% | 90.00% | 90.00% |

### Why Belief-State Veyra Outperforms Cautious B6:
Cautious B6 is strictly siloed: it only queries verification probes. When a non-idempotent write fails and has **no verification probe**, cautious B6 is forced to abstain (`DEFER`).  
In contrast, Belief-State Veyra's unified action space discovers alternative safe recovery pathways:
- If a probe is absent but an **idempotency key is supported**, Veyra executes `IDEMPOTENCY_REPLAY`.
- If a batch mutation partially commits, Veyra executes `RECONCILE` or `COMPENSATE`.  
This multi-pathway recovery accounts for the **+6.08 pp lift over cautious B6** while strictly maintaining the zero-duplication invariant.

---

## 4. $2 \times 2$ Factorial Study: Evidence $\times$ Belief Decomposition

To isolate whether the lift comes simply from evidence acquisition or from partial-observability belief updates, we evaluated all 4 orthogonal configurations ($N=100$ scenarios):

```text
                     Belief Modeling OFF          Belief Modeling ON
               ┌─────────────────────────────┬─────────────────────────────┐
Evidence OFF   │  Cell A: 13.0% Safe Rec     │  Cell C: 43.0% Safe Rec     │
               │  (DER: 0.0%, Abstain: 87%)  │  (DER: 0.0%, Abstain: 56%)  │
               ├─────────────────────────────┼─────────────────────────────┤
Evidence ON    │  Cell B: 39.0% Safe Rec     │  Cell D: 86.0% Safe Rec     │
               │  (DER: 0.0%, Abstain: 58%)  │  (DER: 0.0%, Abstain: 11%)  │
               └─────────────────────────────┴─────────────────────────────┘
```

- **Main Effect of Evidence Acquisition:** **+34.50 pp**
- **Main Effect of Belief Modeling:** **+38.50 pp**
- **Interaction (Synergy):** **+17.00 pp**
- **Utility Function Contribution:** Exactly **0.0 pp**. Disabling utility weights and using uniform costs yielded identical recovery and safety (86.0% rec, 0.0% DER). The lift comes from **epistemic belief updates + the hard risk constraint**, not cost tuning.

---

## 5. Adversarial Epsilon Sweep & Noise Boundary

When tested against **imperfect probe reliability ($P(\text{probe correct}) = 0.95$)**, the true safety-completion frontier appears:

```text
Threshold ε:       0.001   0.010   0.020   0.030   0.040   0.050   0.060   0.080   0.100   0.200
Safe Recovery (%): 61.0    61.0    59.0    61.0    59.0    62.0    73.0    74.0    73.0    76.0
DER (Duplicate %):  0.0     0.0     0.0     0.0     0.0     0.0     1.0     0.0     1.0     2.0
Abstention (%):    37.0    38.0    39.0    37.0    39.0    36.0    25.0    25.0    23.0    18.0
```

- **Safe Regime ($\epsilon \le 0.05$):** Enforces 0.0% duplicate effects even under noisy evidence.
- **Phase Transition ($\epsilon \ge 0.06$):** The controller begins gambling on ambiguous evidence, triggering immediate duplicate mutations (1.0% to 2.0% DER). Setting $\epsilon = 0.01$ is solidly within the zero-duplication boundary.

---

## 6. Simple Probabilistic Model vs Out-Of-Distribution (OOD) Prior Shift

We trained an SGD Logistic Regression baseline on 1,500 scenarios using identical boundary features, then evaluated it alongside Veyra under an **OOD prior shift** (where unprobed `PROCESS_CRASH` failures committed state):

| Setting ($N=500$) | Policy Arm | Safe Recovery Rate | Duplicate Effect Rate (DER) | Safety Outcome |
| :--- | :--- | :---: | :---: | :--- |
| **In-Distribution** | Logistic Regression | 92.00% | 0.00% | Exploited empirical correlations |
| *(Seeds 1 to 10)* | Belief-State Veyra | 86.00% | 0.00% | Conservative, risk-bounded |
| **OOD Prior Shift** | Logistic Regression | 86.00% | **6.00%** | **FAILED (Overfit prior caused duplicate writes)** |
| *(Crashes mutated state)* | Belief-State Veyra | 86.00% | **0.00%** | **PASSED (Epistemic constraint blocked blind retry)** |

---

## 7. LIMBO Late-Commit Benchmark: Observation Equivalence

Evaluated on **100 observation-equivalent late-commit scenarios** where a verification probe reports `committed=False` (due to read replica lag or commit delay), but the transaction committed:

| Late-Commit Condition ($N=100$) | Controller Arm | Action Selected | Safe Recovery Rate | Duplicate Effect Rate (DER) |
| :--- | :--- | :--- | :---: | :---: |
| **Late Commit WITHOUT Idempotency** | Verify-Before-Retry (B6) | Blind `RETRY` | 0.0% | **100.0% (Catastrophic Dupes)** |
| *(Probe reports not committed)* | Belief-State Veyra | Safe `DEFER` | 0.0% | **0.0% (Safe Abstention)** |
| **Late Commit WITH Idempotency** | Verify-Before-Retry (B6) | Blind `RETRY` | 0.0% | **100.0% (Lacks contract hook)** |
| *(Contract provides Idempotency Key)* | Belief-State Veyra | `IDEMPOTENCY_REPLAY` | **100.0%** | **0.0% (Perfect Deduplication)** |

---

## 8. Architectural Positioning & Next Validation Phase

### Architectural Separation:
- **Cordon:** Task-scoped transaction containment & cross-step invariant boundaries.
- **Veyra:** Post-proposal execution control & risk-constrained belief-state recovery.

### Real-Tool Validation Matrix:
The algorithm is frozen. The next phase tests this exact controller on live tools:
1. **PostgreSQL Client:** `UPDATE` $\to$ connection reset (socket drop after commit).
2. **Git Version Control:** `commit` / `push` $\to$ process failure / lost remote ACK.
3. **MCP Filesystem:** `write_file` $\to$ stale cache / dropped response.
4. **Payment Simulator:** `charge` $\to$ lost HTTP ACK / late commit.
