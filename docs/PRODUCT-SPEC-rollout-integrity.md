# Product spec: a rollout integrity layer for RL post-training

**Question this answers:** can the rollout-verification research become a real
production layer that helps part of the training loop?
**Answer:** yes -- and the reason it can is a specific design choice, not
enthusiasm. Sections 4 and 5 are that reason.
**Companion docs:** `DIRECTION-RESEARCH-2026-09-28.md` (why this direction),
`RESEARCH-PLAN-2026-09-28.md` (the JEPA paper track).
**Date:** 2026-09-29.

---

## 1. What already exists upstream (so this is not a duplicate)

Checked directly in `vllm-project/vime` on 2026-09-29:

| existing | where | what it does | what it does not do |
|---|---|---|---|
| rollout metrics | `vime/observability/rollout_metrics.py` | response lengths, pass rate, **`has_repetition`**, vLLM perf fields | repetition is *logged*, not enforced |
| rollout data schema | `vime/observability/rollout_data_utils.py` | declares dtypes for `tokens`, `loss_masks`, `rollout_log_probs`, `rollout_top_p_token_ids/offsets`, `teacher_log_probs`, `rollout_routed_experts` | **no provenance or version key anywhere in the schema** |
| one replay validator | same file, `validate_rollout_routed_experts_for_replay` | validates MoE routing data for replay | narrow: one field, one purpose |
| dynamic sampling filters | `vime/rollout/filter_hub/dynamic_sampling_filters.py` | drops groups with zero reward std | a training-signal filter, not an integrity check |
| rollout contract | `vime/rollout/base_types.py` | `RolloutFnTrainOutput(samples, metrics)` | no version, no engine identity, no manifest |
| weight version tracking | vime #360 (`/pull_weights`) | tracks a disk weight version | framework-internal, not verifiable from outside, not cross-engine |
| engine introspection | vLLM `entrypoints/serve/dev/server_info`, Rust `routes/server_info.rs` | server-level info | **no "which weight version are you serving" answer** |

The pattern: adjacent primitives exist as **metrics and narrow validators**. The
**verification and audit** layer does not exist, and the rollout data contract
carries no provenance at all. That is the opening, and it is narrow enough to be
precise about.

---

## 2. Where the layer sits

    trainer (verl | vime | slime | SkyRL)          engines (vLLM | SGLang)
          |                                                   |
          | 1. push weights, version v ----------------------> |
          |                                                   |
          | 2. nonce challenge -----------------------------> |  computed over k sampled
          |    <----------------------- fingerprint(v)         params AND buffers
          |    verify against trainer's own fingerprint
          |                                                   |
          | 3. rollout request, header {v, nonce} ----------> |
          |    <--- rollouts tagged {engine_id, v} ---------- |
          |                                                   |
    +-----v----------------------+
    |    INTEGRITY LAYER  <-- this tool
    |  provenance | schema & alignment | degeneracy
    |  reward divergence | staleness audit
    +-----+----------------------+
          |  admit | quarantine | gate
          v
    advantage estimation --> optimizer

It sits **between the collected rollout batch and advantage computation**. Nothing
upstream of it, nothing inside the inference kernel, nothing in the backward pass.
That placement is what keeps it cheap and non-invasive.

---

## 3. The five checks

1. **Provenance / version.** Does the engine serve the weights the trainer thinks it
   pushed? Targets the vime #374 / #388 / #446 class directly.
2. **Schema and alignment.** Declared dtype and length invariants, logprob/token
   alignment, shift correctness, mask consistency. Targets vime #426.
3. **Degeneracy.** Repetition, entropy collapse, truncation artefacts, degenerate
   tool-call loops -- sequence statistics on data already in memory.
4. **Reward-proxy divergence.** Calibrated detection that the visible reward is
   drifting from intended capability.
5. **Staleness audit.** Claimed off-policy gap per token versus the gap implied by
   the recorded version tags.

Plus two things that turn checks into a product:

- **Canary suite.** A fixed small prompt set (8-32) whose output statistics must
  hold across a weight update. Turns silent corruption into a seconds-scale signal
  at sync boundaries.
- **Replay.** Deterministic re-execution of a flagged rollout for debugging.

---

## 4. The core primitive: nonce-based parameter challenge-response

This is the piece that makes the tool viable rather than aspirational, and it needs
to be right.

**The problem.** To know that an engine serves version `v`, the obvious approach is
to hash the weights. That fails on two counts: it costs a full pass over GBs, and it
requires transferring weights that efficient sync schemes (SparseRL-Sync's ~100x
communication reduction) are specifically designed not to transfer.

**The approach.** A challenge-response over a *random sample* of parameters:

    trainer -> engine:  { nonce, k sampled slice indices }
    engine  -> trainer: c = sum_i w_i * (nonce . slice_i).sum()

- `k` slices are a few MB, not GBs. The challenge rides the existing sync channel.
- The trainer computes the same `c` from its own copy and compares.
- Random, nonce-dependent index selection defeats replay and make the check
  resistant to a component that caches a previous answer.

**What it detects.** Stale version, partially applied deltas (some layers updated,
others not), dtype/precision corruption, and a wrong model served on an endpoint.

**Two design details that matter and are easy to get wrong:**

1. **Buffers, not just parameters.** The challenge must sample
   `named_parameters()` **and** `named_buffers()`, including non-persistent
   buffers. The stale-RoPE-cache failure (vime #446, and the vLLM patch in #326 to
   preserve non-persistent buffers on layerwise reload) is a *buffer* bug. A
   parameters-only checksum would pass while the rollouts were garbage.
2. **Numeric tolerance, not exact equality.** bf16 reduction order differs between
   trainer and engine. The checksum must be quantised or compared with a tolerance
   calibrated so that a real fault is far outside the noise floor. Calibrating that
   threshold against the observed spread is a required measurement, not a constant
   to guess.

---

## 5. Runtime cost

The adoption question is entirely about overhead. Estimated:

| component | cost | frequency |
|---|---|---|
| challenge-response | MB-scale transfer + O(k) tensor slices | per weight sync |
| schema/alignment/degeneracy | tensor ops over data already in host memory | per rollout batch |
| canary suite | sub-second on a small model | per sync boundary (opt-in) |
| reward-proxy divergence | running statistics, no model calls | online, constant memory |

Target: **under 1 % of step time**, off the critical path. Compare against a measured
step of 3.32-18.85 s in vime #445; a per-step batch check is milliseconds.

If any check ever exceeds that budget, it degrades to sampled execution (check every
N-th batch) rather than slowing the loop. A verification layer that costs throughput
will not be adopted, regardless of what it catches.

---

## 6. Failure semantics: three modes

This is the product decision that determines adoption.

| mode | behaviour | who uses it |
|---|---|---|
| **observe** (default) | log, emit metrics, never block, never change training | everyone, from day one |
| **quarantine** | mark or drop bad groups before the advantage estimator | after trust is earned |
| **gate** | block the step or abort the run | CI, canary failures, opt-in |

Defaulting to observe is what makes the tool safe to install. Gate is opt-in because
a false positive that kills a 20-hour run is worse than the bug it was hunting.

**Honest caveat on quarantine.** Dropping data changes the gradient. If the
quarantine decision correlates with the reward -- and degeneracy filters often do --
you bias the estimator rather than protect it. So the tool must always report the
quarantine rate *and* its correlation with reward, so the user can see when filtering
has become a policy intervention.

---

## 7. What it concretely helps inside training

1. **Before rollouts:** verify the engine's served version, so a corrupt sync cannot
   silently poison a whole rollout round.
2. **After rollouts, before advantages:** validate schema, alignment, and degeneracy,
   so bad tokens never reach the estimator.
3. **Inside off-policy correction:** supply the *measured* per-token version distance.
   Thirteen papers since 2026-05 (staleness-adaptive trust regions, staleness-LR
   scaling laws, freshness-aware control) consume a staleness number; this is the
   only component that produces a verified one. That is the research-to-product
   bridge.
4. **Reward pipeline:** early warning when the proxy and the intended capability
   diverge.
5. **CI and reproducibility:** canary pre-flight after any change to the rollout path,
   plus a per-run manifest making a training run auditable after the fact.
6. **Debugging:** replay a flagged rollout instead of guessing.

---

## 8. What this product cannot do (limits to state up front)

- **It detects; it does not repair.** Localising a bad sync is the deliverable, not
  fixing the sync backend.
- **It needs a small amount of cooperation.** Passing a version tag and answering a
  challenge requires a few lines in the framework or engine adapter. Where no
  cooperation is possible, it degrades to observe-only checks that use artefacts
  already present (recomputing logprobs from tokens, sequence statistics).
- **Quarantine can bias training** (section 6).
- **The provenance half commoditises** if frameworks ship native version reporting
  (vime #360 is moving that way). The durable parts are cross-framework coverage,
  tamper-evidence, and the detection algorithms.
- **Reward-proxy divergence is the least proven check.** It is scored last on
  purpose, and R4 in the research plan pre-registers a likely negative result.

---

## 9. Productisation paths

| path | model | analogy |
|---|---|---|
| OSS core + hosted dashboard | open the checks, sell the fleet view and history | langfuse (35k stars) |
| "CI for training runs" | sell the canary suite + regression gate to labs | CI vendors |
| Pure OSS + paper | credibility and employability, no revenue | most research tooling |

The buyer for the paid paths is whoever is currently losing GPU-days to a run that
quietly trained on corrupted rollouts. In a 100-GPU post-training job, one wasted
day dwarfs the price of the tool, which is the whole pitch.

---

## 10. Honest competitive position

| adjacent work | scope | differentiation |
|---|---|---|
| vime `observability/` + `filter_hub` | metrics, narrow validators, training-signal filters | provenance, enforcement, cross-framework, cross-run |
| vime #360 `/pull_weights` version tracking | internal, disk-based, framework-specific | externally verifiable, engine-agnostic, tamper-evident |
| WeightBridge (2609.25442), SparseRL-Sync (2605.07330) | make transfer fast | verify it landed, per batch |
| BenchShield (2609.11028) | reward integrity for *evaluation* | runtime integrity for *training*; borrow its lifecycle-model framing |
| HackProbe (2609.04665) | reward hacking in *self-evolving* loops | RL rollout batches, framework-integrated; borrow its p-value calibration |
| langfuse / mlflow | generic LLM observability | RL-training-specific invariants they will not build |

---

## 11. MVP, in the order that de-risks fastest

| step | deliverable | needs GPU? | gate |
|---|---|---|---|
| 0 | fault harness reproducing three signatures: garbled repetition after a stale update, stale RoPE cache, misaligned logprobs | **no** | each fault reproducible and detectable offline |
| 1 | reference implementation of the checks + a CLI over recorded rollout data | no | precision/recall on injected faults |
| 2 | canary suite format + challenge-response prototype against a local engine | one small model | detects a deliberately stale engine, zero false positives on a clean run |
| 3 | one framework adapter (vime or verl), observe-only | reuse the above | a maintainer takes it seriously |
| 4 | report / paper, then quarantine and gate modes | -- | -- |

Step 0 is the credibility anchor and needs no GPU, no cluster, and nobody's
permission. If the three signatures cannot be reproduced, the premise is wrong and
that is learned for free.

---

## 12. Interface sketch

The layer should be usable two ways, because that decides adoption:

    # library, in-process
    from rollout_integrity import Auditor, policy
    auditor = Auditor(checks=["provenance", "alignment", "degeneracy"],
                      mode=policy.OBSERVE)
    report = auditor.audit(batch, expected_version=v)   # batch: the framework's dict
    report.admitted, report.quarantined, report.metrics

    # CLI, over recorded data -- works with no framework integration at all
    rollout-integrity audit --rollouts run/rollout_0007.pt \
        --manifest run/manifest.json --report report.html

The CLI path matters: it lets someone evaluate the tool on data they already have
before touching their training loop.

---

## 13. Next action

Build step 0. It is CPU-only, needs no permissions, and it either reproduces the
failure class from the vime issues or it does not. Everything above is contingent on
that result.