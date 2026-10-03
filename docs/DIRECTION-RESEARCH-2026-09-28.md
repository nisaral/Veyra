# Direction research: where an individual can still stand out

**Scope:** JEPA / world models, agents, and RL rollouts -- plus which of these can
become an actual product.
**Date:** 2026-09-28. All numbers below were pulled live from the GitHub and arXiv
APIs during this session and are reproducible.

---

## 1. Method

Four sweeps, none of which is vibes:

1. **Repo scale and velocity** for every serious project in each layer
   (`/repos/{owner}/{repo}`).
2. **Issue-tracker archaeology** on the two rollouts projects, to distinguish
   "roadmap is internally owned" from "here is an open, unclaimed problem".
3. **arXiv sweeps** by `abs:` query, sorted by submission date, to find the current
   research frontier and who has already claimed what.
4. **GitHub saturation counts** (`/search/repositories`) for the *tooling* terms,
   which is the measurement that matters for "is this built yet?". Star ceilings are
   the tell: a term with 700 results and a 12-star maximum is an unbuilt space, not a
   crowded one.

---

## 2. The landscape, by layer

| layer | who owns it | size evidence | open to an individual? |
|---|---|---|---|
| Base models | frontier labs | -- | no |
| Inference engines | vLLM, SGLang | 92.9k / 36.5k stars | no |
| Rollout routing | vllm-project/router | 443 stars, 127 issues, moved 2026-09-24 | marginal |
| RL training frameworks | verl, TRL, OpenRLHF, slime, AReaL, SkyRL, prime-rl, NeMo-RL | 23.7k / 19.4k / 10.0k / 8.6k / 5.8k / 2.4k / 2.1k / 2.0k stars | no (saturated) |
| Weight transfer | WeightBridge (arXiv:2609.25442) | new, funded | no |
| Env lifecycle for agentic RL | WeEnv (arXiv:2609.30766, WeChat) | new, funded | no |
| Staleness tolerance | ~13 papers since 2026-05 | active research | partly |
| Credit assignment, agentic RL | SLCA-GRPO, trajectory graphs, PACT | ~4 papers in Sept 2026 alone | partly |
| **Verification / validation of rollouts** | **nobody** | **see section 5** | **yes** |

The pattern is unambiguous: the *plumbing* layer is being industrialised at speed by
well-resourced labs, and the *verification* layer above it does not exist.

---

## 3. Thread 1 -- JEPA / world models

**What we have.** `docs/RESEARCH-PLAN-2026-09-28.md`, with a runnable theory
artifact (`theory/head_collapse_identifiability.py`) establishing that the stock
EB-JEPA objective forces head disagreement to zero, that the epistemic/aleatoric
split is unidentified without replicates, and that shared-encoder drift makes the
divide-by-M uncertainty estimate inconsistent with bias *increasing* in M.

**Honest assessment.** The science is good and the C3 result is genuinely novel. But
it is a **paper track, not a product**:

- The ceiling is the 97 +/- 2 % Two Rooms baseline; the payoff has to be found in
  calibration and compute-allocation metrics.
- Nothing about it is sellable. Nobody pays for a JEPA calibration fix.
- The upstream repo is a Meta research library at 790 stars -- the audience for the
  contribution is small.
- It needs GPU hours to produce any empirical result at all.

**Verdict:** keep it. It is a legitimate first-author paper and the pre-registration
discipline carries over. Do not make it the flagship.

---

## 4. Thread 2 -- agents

| segment | state | evidence |
|---|---|---|
| Observability | mature and owned | langfuse 35.1k, mlflow 28.2k |
| Sandbox / execution | taken | microsoft/agent-governance-toolkit 6.3k, moltis 2.9k, arrakis 882, SWE-ReX 606 |
| Context engineering | crowded with docs, not code | how-claude-code-works 3.7k, Awesome-Context-Engineering 3.3k, harness-books 3.1k, context-engineering-kit 1.7k |
| Eval harnesses | fragmented, small | claw-eval 779, HarnessEval-W 301, SanityHarness 242 |
| Agent regression testing | mislabelled | 712 repos, but the leaders (Tracely-ai 1.4k, qa-agent 1.3k) are QA-for-web-apps, not regression of *agent runs* |
| Agent replay / debugging | nearly empty | best hit is Chidori 1.4k (durable execution); the rest are 269 stars and down |

The agent-side product surface is picked over. The one live gap -- regression and
replay of agent runs -- is the same gap as the rollout side, seen from the other end.

---

## 5. Thread 3 -- rollouts: the finding

### 5.1 The plumbing is being built by industry labs, right now

vime's open issues are a roadmap being executed at speed, largely by the core team
(most issues carry exactly one maintainer comment -- these are mirrored internal
plans, not community requests):

- weight sync: #360 full-disk `/pull_weights` (9 comments, the most-discussed),
  #335 NCCL m2n backend, #392 direct NCCL delta updates, #425 S3 delta backend,
  #447 sparse HCCL sync
- colocate cost: #442 RFC "colocate offload/onload dominates step time on
  small-model short-response workloads"
- **a measured 5.7x**: #445 documents that skipping the forced offload took a step
  from **18.85 s to 3.32 s**
- observability: #437, #439, #440, #443 filling in spans and transfer metrics

Combined with WeightBridge, WeEnv, RolloutPipe, ECHO-2, SparseRL-Sync, and
vllm-project/router, the conclusion is that *building rollout plumbing as an
individual in 2026 is a losing bet*.

### 5.2 But the bugs that matter are silent, and unowned

From the same tracker, a different class of issue:

| issue | symptom |
|---|---|
| #374 | garbled and repetitive MoE rollouts after a colocated IPC weight update |
| #388 | garbled rollout outputs on 4x8 A800 with EP4/PP2/TP2; "HF1 != HF1-Megatron-HF2" |
| #446 | broken rollouts because the RoPE cos/sin cache was not restored after a weight update |
| #426 | sampled-token log probabilities missing or misaligned |

Every one of these is a case where **the training loop kept running while the
rollouts were wrong**. The failure is not a crash; it is silently degraded data
being fed into the optimizer. That is the most expensive failure mode in the whole
stack, and there is no tool that catches it.

### 5.3 The research frontier assumes the measurement is correct

Thirteen papers since 2026-05 work on *tolerating* staleness: Staleness-Adaptive
Trust Regions (2607.18722), Staleness-LR Scaling Laws (2607.01083), entropy-scaled
trust regions (2607.22186), freshness-aware control (2605.17862), AsyncOPD
(2606.24143). Others make transfer fast: SparseRL-Sync claims ~100x less
communication (2605.07330).

All of them take it as given that the *actual* policy version and the *actual*
off-policy gap are known. Nobody verifies that. `Missing Old Logits in Asynchronous
Agentic RL` (2605.12070) repairs off-policy correction *given* staleness -- it does
not check the staleness number itself.

### 5.4 The tooling gap, measured

GitHub search, live:

| query | repos | top result |
|---|---:|---|
| `weight sync verification` | 2 | 0 stars |
| `training data integrity reinforcement learning` | 1 | -- |
| `llm training run debugging` | 3 | 1 star |
| `canary regression llm testing` | 3 | 0 stars |
| `rollout validation` | 97 | 5 stars |
| `reward hacking detection` | 21 | 21 stars |
| `llm output validation framework` | 78 | 16 stars |

Two adjacent papers landed this month, which validates the space while leaving it
open:

- **BenchShield** (arXiv:2609.11028) -- reward integrity for *evaluation*
  infrastructure, via a finite lifecycle model of reward-relevant events. Research
  prototype; not runtime training; not a tool.
- **HackProbe** (arXiv:2609.04665) -- harness-agnostic reward-hacking monitor for
  *self-evolving* loops, attaching through two black-box hooks with a frozen
  comparison core and calibrated family-wise p-values. Not RL rollout batches; not
  integrated with any training framework.

---

## 6. Recommendation: rollout integrity -- the verification layer

> Everyone is building faster plumbing. Nobody is building the brake lights.

**The artefact:** an adoptable, framework-agnostic integrity layer that sits on the
rollout batch and answers one question per step: *was the data that just entered the
optimizer the data the trainer thinks it was?*

### 6.1 The five checks

1. **Provenance and version.** Every rollout batch carries a manifest naming the
   weight version that actually produced it, checked against the trainer's declared
   version. Directly targets the vime #374/#388/#446 class.
2. **Token / logprob alignment invariants.** Length, shift, and logprob consistency
   between the sampler's output and what the trainer consumes (#426).
3. **Degeneracy.** Repetition, entropy collapse, truncation artefacts, empty tool
   results -- cheap sequence statistics.
4. **Reward-proxy divergence.** Calibrated detection that the visible reward is
   drifting from the intended capability (HackProbe's calibration idea, applied to
   RL training rather than self-evolution).
5. **Staleness accuracy.** Claimed off-policy gap versus measured, which is exactly
   the input every staleness-adaptive method above assumes to be trustworthy.

Plus a **canary suite**: a fixed set of prompts whose output statistics must remain
stable across a weight update. It turns "silent corruption" into a failure that
surfaces in seconds instead of hours, and a **replay** path for a flagged rollout.

### 6.2 Why this is the standout choice

- It is the **reverse of the crowded direction**. The labs are all racing to make
  transfer faster; the value of a verification layer *increases* as the plumbing
  gets more asynchronous and more complex.
- **It fits this machine.** All five checks are CPU-only, deterministic, and
  unit-testable. A GPU box is needed only to generate a rollout corpus, on small
  models (vime's own spikes use Qwen3-4B GRPO).
- **It is productizable.** "Why did my run break / why did my reward go sideways"
  is something a lab will pay for, and it is the same shape as the observability
  products that already succeeded (langfuse at 35k stars proves the appetite).
- **It is a paper.** Section 6.3.
- **It extends DIO rather than restarting it.** DIO is already an SLO-aware routing
  gateway that observes backend behaviour; a rollout-integrity layer is the same
  competency applied to the training loop instead of the serving path.
- **Failure is cheap.** Each check is independently useful, so the project degrades
  gracefully instead of dying whole.

### 6.3 Falsifiable research claims

| id | claim | falsified if |
|---|---|---|
| R1 | a measurable fraction of step-to-step loss/reward variance is explained by version skew that the trainer's own logs do not show | version skew explains < 5 % of variance in a controlled sweep |
| R2 | canary statistics detect weight-sync corruption in O(seconds) versus O(hours) to notice via loss | loss detects it equally fast on injected faults |
| R3 | calibrated reward-proxy divergence predicts reward-hacking onset before it is visible in held-out eval | no lead time on a controlled proxy-gap setup |
| R4 | most "reward hacking" alarms in a healthy run are false positives, and the honest false-positive rate is reportable | calibrated detector has no useful operating point |

R4 is deliberately a negative result, in the same spirit as the JEPA plan's H6.

### 6.4 Compute

- Checks, canaries, replay, corpus analysis: **CPU only**, runs on this machine.
- Corpus generation: reuse the Kaggle 2xT4 path, small models. A few thousand
  rollouts is enough for R1/R2; R3 needs a proxy-gap environment.
- Phase 0 needs **no GPU at all** (see below), which is the point.

### 6.5 Phases

| phase | deliverable | gate |
|---|---|---|
| 0 | reproduce three known failure signatures in a tiny local harness: garbled repetition after a stale update, stale RoPE cache, misaligned logprobs | each fault is reproducible and detectable offline |
| 1 | provenance manifest + canary suite, framework-agnostic | canaries flag an injected fault, zero false positives on a clean run |
| 2 | degeneracy + alignment detectors, measured precision/recall on injected faults | precision and recall reported, not asserted |
| 3 | reward-proxy divergence with calibrated statistics | R3 lead time measured |
| 4 | one real adapter (vime or verl) + upstream discussion | maintainer response |
| 5 | paper | -- |

Phase 0 is the credibility anchor and costs nothing: it demonstrates the failure
class exists without needing anyone's permission or any GPU time.

### 6.6 Positioning against the adjacent work

| existing | scope | what we do differently |
|---|---|---|
| WeightBridge, SparseRL-Sync, RolloutPipe, vime #360 | make transfer fast and roughly correct | verify that it *was* correct, per batch |
| BenchShield (2609.11028) | reward integrity for *evaluation* | runtime integrity for *training* rollouts; borrow their lifecycle-model idea |
| HackProbe (2609.04665) | self-evolving loops, black-box hooks | RL rollout batches, framework-integrated; borrow their p-value calibration |
| Staleness-adaptive methods (2607.18722, 2607.01083, ...) | tolerate a staleness number | verify the staleness number they consume |

---

## 7. Scoring of all candidates

Scored 1-5; "individual" = can one person ship it without a cluster or a team.

| direction | novelty | individual | compute fit | product | uncrowded | total |
|---|---:|---:|---:|---:|---:|---:|
| **Rollout integrity layer** | 4 | 5 | 5 | 4 | 5 | **23** |
| JEPA uncertainty (current plan) | 5 | 4 | 3 | 1 | 4 | 17 |
| Reward-hacking monitor alone | 3 | 5 | 5 | 3 | 3 | 19 |
| Credit assignment for agentic RL | 3 | 2 | 2 | 2 | 2 | 11 |
| Agent sandbox / execution governance | 2 | 3 | 4 | 4 | 2 | 15 |
| Agent eval harness | 2 | 3 | 4 | 4 | 2 | 15 |
| Rollout routing / gateway (DIO v2) | 2 | 3 | 4 | 3 | 1 | 13 |
| Context / memory tooling | 2 | 4 | 5 | 3 | 1 | 15 |

Note that "reward-hacking monitor alone" scores well but is a strict subset of the
recommendation, and it is the one segment with a fresh academic entrant. Bundling it
into the integrity layer is strictly better.

---

## 8. Risks

| risk | mitigation |
|---|---|
| Frameworks change fast; adapters rot | keep adapters thin against the rollout-batch dict contract, not internals |
| HackProbe/BenchShield expand into this scope | differentiate on runtime training + integration; cite them, do not compete |
| Corpus generation needs GPU | Phase 0 needs none; R1/R2 need a few GPU-hours on small models |
| "Verification" sounds unglamorous | the pitch is the failure mode, not the tool: silent corrupted rollouts burning GPU-weeks |
| JEPA plan competes for attention | it is a separate paper track; keep it on the shelf, do not parallelise the build |

---

## 9. Immediate next actions

1. **Phase 0, this week, zero GPU:** write the tiny harness that reproduces the
   three failure signatures from the vime issues. If these cannot be reproduced,
   the premise is wrong and we learn that for free.
2. **Write the R1-R4 pre-registration** before any measurement, matching the
   discipline already used in `uncertainty-diffusion-world-models/research/`.
3. **Decide the name and ship a skeleton repo** with the check interface, the
   canary suite format, and one adapter stub.