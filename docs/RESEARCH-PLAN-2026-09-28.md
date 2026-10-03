# Uncertainty-Preserving JEPA Predictor Ensembles

**Research plan and implementation spec**
Target upstream: [`facebookresearch/eb_jepa`](https://github.com/facebookresearch/eb_jepa) (`examples/ac_video_jepa`)
Date: 2026-09-28
Status: Phase 0 complete (theory artifact runnable). Phases 1-6 specified, not yet run.

---

## 0. Thesis

A JEPA predictor is an ensemble of M >= 2 prediction heads over a shared
encoder. The obvious way to get uncertainty out of it -- spread the heads
apart and read their disagreement -- does not work, and this document shows
*why* it cannot work, *what* fixing it requires, and *what new obstruction
appears* that is specific to JEPA and does not exist in the
teacher-student/ensemble distillation setting where this problem was
previously studied.

The contribution has two halves:

1. A **negative/theoretical** result with a constructive fix: cross-head
   disagreement is driven to *exactly zero* by the stock objective, and the
   epistemic/aleatoric split it is supposed to encode is provably
   unidentified under any sum-only objective. Both are demonstrated
   numerically (`theory/head_collapse_identifiability.py`) and are testable
   against the real codebase.
2. A **JEPA-specific** result: because the prediction target is the encoder's
   own output and the encoder moves during training, the M heads are *not
   exchangeable*. The standard "divide the spread by M" uncertainty estimate
   is therefore inconsistent, and the bias *grows* with M. This means more
   ensemble members make JEPA uncertainty estimates worse, not better.

The payoff experiment -- calibration-gated planning compute -- is deliberately
chosen so that a null result is still a publishable finding rather than a
dead project (see section 8).

---

## 1. Corrections to the previous draft of this plan

The earlier version of this proposal had the right target and the right gap.
Four things in it were wrong or missing, and they matter.

**1.1 The January-2026 paper is VJEPA, not "BJEPA".**
`VJEPA: Variational Joint Embedding Predictive Architectures as Probabilistic
World Models`, arXiv:2601.14354v1 (2026-01). Searches for "BJEPA" return that
paper, which is why the name propagated. Cite the real title.

**1.2 Two Rooms is a deterministic dynamical system.** The earlier plan assumed
environment stochasticity as the source of the aleatoric term. There is none:
`eb_jepa/datasets/two_rooms/env.py::_generate_transition` is
`next_location = location + action`, and `step` (env.py:135) calls
`_calculate_next_position` (env.py:182) which is that plus a wall check.

The randomness that does exist is somewhere else, and locating it correctly is
what makes the identifiability fix implementable:

- **Action-sampling noise.** `dot_dataset.py::generate_actions` draws each step
  angle from a Von Mises centred on the *previous* angle with concentration
  `1 / action_angle_noise`, and each step length from a truncated normal
  (`loc=action_step_mean=1.0`, `scale=action_step_std=0.4`, bounds
  `[action_lower_bd, action_upper_bd] = [0.2, 1.8]`). With
  `action_angle_noise = 0.2` (`data_config.yaml`) this is large.
- **Wall-bounce noise.** `datasets/two_rooms/utils.py::check_wall_intersect`
  (utils.py:213, `add_noise=True`) adds `torch.randn(2) * 0.5` to the
  intersection point (utils.py:259, 276, 309). Env-level, but only on collision.
- **Encoder aliasing.** `E_phi(o)` is not injective: positions closer than the
  rendering resolution (`img_size=65`, `dot_std=1.3`) can map to
  indistinguishable latents, so `E(o_{t+1})` has conditional variance given
  `(E(o_t), a_t)` even though the true state is deterministic.

**1.3 The previous plan's headline metric has no headroom.** Upstream reports
**97 +/- 2 %** success (MPPI) and 96 +/- 2 % (CEM) on Random Wall, averaged over
3 seeds, 3 epochs and N=20 episodes (`examples/ac_video_jepa/README.md`). A
project whose headline is "+SR" is pre-doomed at that ceiling. Section 8 picks
metrics that can actually move.

**1.4 The two structural facts that make this a research problem were missed.**

- The prediction loss is *separable across heads*: in
  `eb_jepa/jepa.py::JEPA.unroll`, each head contributes
  `self.predcost(state, predicted_states)` against the same target. Nothing in
  the objective couples the heads, so the only global optimum has every head at
  the conditional mean. Collapse is not an accident of optimization; it is the
  optimum.
- The anti-collapse regularizer never sees the predictor. In `unroll` it is
  called as `self.regularizer(state, actions)` where
  `state = self.encoder(observations)`. VICReg-style variance/covariance terms
  (`eb_jepa/losses.py`) act on *encoder* features and the IDM term acts on
  consecutive encoder states. Consequently **head-collapse is invisible to the
  entire regularizer**: an ensemble of M identical heads incurs zero
  regularization penalty.

That second point is the crux. It is also why the fix cannot be "add VICReg to
the heads" without an argument about *which* variance is being preserved.

---

## 2. What the code actually does (constraints any solution must respect)

Verified against upstream `main` on 2026-09-28.

| Fact | Where | Why it constrains us |
|---|---|---|
| `self.single_unroll = getattr(self.predictor, "is_rnn", False)` | `jepa.py::JEPAbase.__init__` | An ensemble wrapper **must** forward `is_rnn`, or autoregressive unrolling silently changes semantics |
| `context_length = getattr(self.predictor, "context_length", 0)` | `jepa.py::JEPA.unroll` | Same; `SimplePredictor`/`StateOnlyPredictor` define it |
| `predicted_states = self.predictor(predicted_states, actions_encoded)[:, :, :-1]` | parallel mode | One call in, one tensor out; a multi-head wrapper must reshape without breaking this |
| `pred_step = self.predictor(context_states, context_actions)[:, :, -1:]` | autoregressive mode | Planning path; slices the last timestep |
| `ploss += self.predcost(state, predicted_states) / nsteps` | `unroll`, `compute_loss=True` | `predcost = SquareLossSeq` (MSE). A per-head loss must **sum**, not mean, or it re-normalizes the collapse away |
| `rloss, rloss_unweight, rloss_dict = self.regularizer(state, actions)` | `unroll` | Regularizer sees encoder states only, never `predicted_states` |
| `predicted_encs = self.unroll(obs_init, actions); return self.objective(predicted_encs)` | `planning.py:531` `Planner.cost_function` | Single integration point for an uncertainty-aware objective |
| `GCAgent.unroll` returns `predicted_states` only, `unroll_mode="autoregressive"`, `compute_loss=False` | `planning.py:409-433` | Must gain a `return_heads=True` path for the planner to see per-head latents |
| `diff = MSE(target, encodings).mean(dim=(1, 3, 4))`, optional `diff.sum(dim=1)` | `planning.py:468` `ReprTargetDistMPCObjective` | Where a variance penalty is added |
| CEM returns the refined **mean** of elites | `planning.py:569-658` | At planning time the executed action is deterministic; the aleatoric term is *not exercised* during rollout |

The last row is important and easy to miss. During planning the model is rolled
out under a fixed action, so the aleatoric term contributes nothing to the
rollout distribution. **For planning, only the epistemic term matters, and the
aleatoric term matters only as a nuisance that must be removed from the
ensemble spread.** That is precisely what makes the C3 result central rather
than academic: if the spread is contaminated by a term that is not epistemic and
does not average away, the planner consumes garbage.

Config facts used throughout: `train.yaml` batch 384, `nsteps: 8`, `epochs: 12`,
`Impala` encoder, `henc/hpre/dstc: 32`, `use_amp: true` `dtype: bfloat16`,
`compile: true`; sweep seeds `[1, 1000, 10000]`; regularizer
`(cov, std, sim_t, idm) = (8, 16, 12, 1)`.
`planning_cem.yaml`: `plan_length: 90`, `n_iters: 20`, `num_samples: 200`,
`num_elites: 20`, `var_scale: 1.5`, `max_norms: [2.45]`, `ctxt_window_time: 1`,
`objective_type: repr_dist`, `sum_all_diffs: true`, `goal_source: random_state`.
`eval.yaml`: `num_eval_episodes: 20`, `n_allowed_steps: 200`.
Measured planning cost: **37 s/episode** (both MPPI and CEM).
Upstream issue **#31** records that the defaults assume **~24 GB VRAM**.

---

## 3. The three claims, with measured numbers

Everything below is produced by `theory/head_collapse_identifiability.py`, which
runs on CPU in a few seconds with numpy + scipy only (verified on
Python 3.11.9, numpy 2.1.1, scipy 1.17.1).

### C1 — disagreement is driven to zero, not merely noisy

The stock loss is separable over heads; its global optimum puts every head at
the conditional mean.

| quantity | value |
|---|---|
| mean spread, heads sharing one feature map | **2.658e-33** (the exact optimum) |
| mean spread, heads with independent init | 3.727e-02 |
| mean squared predictive error | 6.582e-01 |
| Spearman(spread, per-sample error) | **+0.036** |

The residual spread under independent initialisation is uncorrelated with the
error it would be used to predict. Disagreement is not a weak signal; at the
optimum it is not a signal.

### C2 — the split is unidentified, and replicates identify it

Training disagreement to match the total predictive variance `v = w + g`
constrains only the sum, so every split in a one-parameter family attains zero
loss:

| alpha | w_s | g_s | sum | naive loss | decision set size |
|---:|---:|---:|---:|---:|---:|
| 0.00 | 0.0000 | 0.9204 | 0.9204 | 0.0 | **1.000** |
| 0.25 | 0.0701 | 0.8503 | 0.9204 | 7.9e-34 | 0.890 |
| 0.50 | 0.1402 | 0.7802 | 0.9204 | 0.0 | 0.625 |
| 1.00 | 0.2804 | 0.6400 | 0.9204 | 0.0 | 0.312 |
| 1.50 | 0.4207 | 0.4998 | 0.9204 | 1.4e-33 | **0.000** |

`alpha = 0` reports zero reducible error and is maximally wrong, at identical
objective value. The last column is the fraction of contexts a planner would
treat as "worth more thought"; it ranges from 100 % to 0 % across a family the
loss cannot distinguish. The unidentified direction is **decision-relevant**.

Two independent transitions from the same `(z_t, a_t)` identify the terms:

| estimate | value | truth | rel. error |
|---|---|---|---|
| aleatoric `g_hat` | 0.6369 | 0.6400 | 4.9e-03 |
| epistemic `w_hat` | 0.2836 | 0.2804 | 1.1e-02 |

and the split loss then separates the family: **1.608e-01 at the collapse
versus 1.982e-05 at the truth (~8100x)**.

**Where the replicates come from in the real code.** This is not a hypothetical
luxury. `dot_dataset.py::generate_actions` draws each realised action as a
commanded direction plus Von Mises angular noise plus truncated-normal step
noise, and the generator **returns `bias_angle`**, the commanded direction,
alongside `actions`. So the conditioning variable is available: condition on
`(E(o_t), bias_angle_t)`, draw two independent noise realisations, and the two
resulting target latents are genuine replicates. Two Rooms supplies the replicate
structure for free; the previous draft never said where its aleatoric term came
from, and the answer turns out to be in the dataset signature.

This also gives a **closed-form cross-check**: the analytic latent variance
induced by `action_angle_noise`/`action_step_std` can be compared against
`g_hat`, which is a cheap falsification of the estimator before any training.

### C3 — encoder drift does not average away (the JEPA-specific obstruction)

The target `E_phi(o_{t+1})` is produced by an encoder that moves during
training. Every standard deep-ensemble uncertainty estimate assumes the members
are exchangeable and divides the spread by M. Because the heads share the
encoder, a shared drift component survives that division. Writing the drift
variance as `c^2`, the fractional **under**-estimation of the true variance of
the ensemble mean is:

| M | c=0.0 | c=0.2 | c=0.5 |
|---:|---:|---:|---:|
| 2 | 0.000 | 0.235 | 0.424 |
| 8 | 0.000 | 0.683 | 0.837 |
| 32 | 0.000 | 0.905 | 0.958 |
| 128 | 0.000 | 0.975 | 0.989 |
| 256 | 0.000 | 0.987 | **0.995** |

With `c = 0` the estimator is exact and M-independent. With `c = 0.5` it
reports essentially **none** of the real variance at M=256 -- and the error
*increases* with M. More heads make the estimate worse, not better. This is the
opposite of the usual ensemble story and it is a direct consequence of the
shared, moving encoder.

Ranking is a separate failure mode from level:

| drift regime | Spearman vs true `w` | misclassified at a clean threshold |
|---|---:|---:|
| no drift | +1.000 | 0.0 % |
| co-monotone with the error | +1.000 | **21.8 %** |
| non-monotone | **+0.661** | 20.2 % |

Co-monotone drift inflates the level but preserves the ordering, so top-k action
selection survives while any *absolute* uncertainty threshold silently
misclassifies a fifth of contexts. Drift whose shape does not match the true
error destroys the ordering, so the planner prefers the wrong actions.

**Statement to be tested on the real model.** The members of a JEPA predictor
ensemble are not exchangeable, because they share a moving encoder; therefore
`Var_i(h_i)/M` is not a consistent estimator of predictive variance, and its
bias is increasing in M.

---

## 4. Hypotheses, each with a falsification bar

| id | hypothesis | falsified if |
|---|---|---|
| H1 | Head disagreement at the stock objective's optimum carries no information about rollout error | Spearman(disagreement, rollout error) > 0.5 on held-out trajectories |
| H2 | A sum-only objective leaves the epistemic/aleatoric split unidentified | Two settings with the same objective value differ materially in calibration |
| H3 | Replicate-conditioned split training identifies the aleatoric term | `g_hat` deviates from the analytic action-noise prediction by more than 20 % |
| H4 | Freezing the encoder on the uncertainty branch reduces the drift bias | Frozen and joint variants show no difference in `d Bias / d M` |
| H5 | Calibrated epistemic uncertainty buys planning compute | No compute reduction at matched success rate |
| H6 | Uncertainty degrades as a risk signal out of distribution | OOD calibration is as good as in-distribution |

H1 and H4 are the ones that decide whether there is a project. H5 is the one
that decides whether there is a *useful* project. H6 is pre-registered as a
likely negative result (see section 9).

---

## 5. Implementation plan

Everything below is shaped to match upstream conventions so the eventual PR is
additive: `CONTRIBUTING.md` requires 2-space indentation and 80-character lines,
and a CLA. Each upstream example is `examples/<name>/{main.py, eval.py, cfgs/}`.

### 5.1 New modules

**`examples/ac_video_jepa_ub/ub_predictors.py`**
`MHeadPredictor(nn.Module)` wrapping M copies of the base predictor
(`ResUNet`/`RNNPredictor`).
- Must expose `is_rnn` and `context_length` (delegating to the wrapped
  predictor), or `JEPA.unroll` and `JEPAbase.__init__` silently change behaviour.
- Two call modes:
  - `heads="batch"`: tile the input into the leading dimension, run once, reshape
    to `[M, B, ...]`. One kernel call; this is the training path.
  - `heads="stack"`: loop, for memory-constrained planning.
- Returns `[M, B, D, T, H, W]`; a compatibility shim returns the mean so the
  unmodified `unroll` still works when `heads=1`.

**`examples/ac_video_jepa_ub/ub_losses.py`**
- `SplitEnsembleLoss`: aleatoric from replicate targets (re-drawn Von
  Mises/truncated-normal actions at the recorded `bias_angle`), epistemic from
  the bias-corrected head disagreement; both terms matched separately. Per-head
  prediction costs **sum**.
- `HeteroscedasticHeadLoss`: Gaussian NLL with a learned per-dimension scale, as
  an alternative parameterisation of the same split.
- `DriftRegulariser`: optional penalty on the temporal derivative of the target
  encoder, to make the drift term `c^2` smaller rather than merely correcting for
  it.

**`examples/ac_video_jepa_ub/ub_planning.py`**
- `GCAgent.unroll(..., return_heads=True)` -> per-head latents.
- `ReprTargetDistUCObjective` implementing the same call signature as
  `ReprTargetDistMPCObjective` (`__call__(encodings, keepdims)`), with
  `cost_t = dist_t + lambda * sigma_t` and a `dist_t / (sigma_t^2 + eps)`
  variant. Consumes a per-head tensor instead of a single one.
- `AdaptiveCEMPlanner` / `AdaptiveMPPIPlanner` subclassing `CEMPlanner` /
  `MPPIPlanner`: the objective returns per-sample uncertainty, and
  `num_samples`/`n_iters`/`num_elites` are re-allocated across the proposal
  distribution by uncertainty. Keeps the existing `PlanningResult` contract.

**`examples/ac_video_jepa_ub/ub_metrics.py`**
Calibration (NLL, ECE, rank correlation against realised error), episode-failure
AUROC from uncertainty *before* commitment, and the compute curve (SR as a
function of planning samples).

### 5.2 Config additions

`cfgs/train_ub.yaml` derived from `train.yaml` plus:

    model:
      n_heads: 4
      split:
        mode: replicate        # replicate | nll | off
        lambda_epistemic: 1.0
        freeze_encoder_for_u: true
      drift:
        penalise_target_velocity: 0.0

Small-GPU overrides kept in a separate `cfgs/train_ub_t4.yaml` (see section 7),
which also addresses upstream issue **#31**.

### 5.3 Non-negotiable integration details

- Heads tiled into the **batch** dimension, not the time dimension, so the
  existing `unroll` slicing (`[:, :, :-1]`, `[:, :, -1:]`) keeps working.
- `head_disagreement` must be computed from the *same* `predicted_states` the
  prediction loss is computed from, not from a second forward pass.
- The unmodified `ac_video_jepa` must keep working; every change is opt-in via
  config. This is also what makes the baseline a fair comparison.

---

## 6. Experiments, with gates and abort criteria

| phase | what | gate to continue | rough cost |
|---|---|---|---|
| 0 | Theory artifact (done) | assertions pass | seconds, CPU |
| 1 | Baseline reproduction at reduced batch | MPPI SR within 5 pts of 97 % | 1 run |
| 2 | Stock objective, M=4, measure disagreement vs rollout error | **if Spearman > 0.5, H1 is dead** -> pivot to C3 only | 1 run |
| 3 | Replicate split training | held-out calibration better than stock | 1 run |
| 4 | Frozen vs joint uncertainty branch | drift bias shrinks when frozen | 2 runs (shared encoder) |
| 5 | Calibration-gated planning | equal SR at lower compute | reuse checkpoints |
| 6 | OOD wall/door stress | pre-registered as likely negative | reuse checkpoints |

Phase 2 is the pivotal one and it is also the cheapest way to lose the project
honestly: if head disagreement already tracks rollout error on the real model,
the central claim is wrong and should be reported as such.

Pre-registration discipline follows the prior project
(`uncertainty-diffusion-world-models/research/*PREREGISTRATION*.md`): write the
bar, then read the number.

---

## 7. Compute budget

**Local.** Only `torch 2.11.0+cpu` is installed. Training locally is out;
analysis, the theory artifact, unit tests and figure generation run locally.
(Note the local torch also explains why `compile: true` and the `2.6.0 -> 2.13.0`
bump issue #26 are irrelevant locally and relevant on the GPU box.)

**GPU.** Kaggle 2xT4 (16 GB each) as used previously. Upstream defaults assume
~24 GB (issue #31), so scale the batch: `batch_size` 384 -> 64 or 96, keep
`nsteps: 8`, `epochs: 12`, `use_amp` bf16, `compile: false` on the T4 path.

**Ensemble overhead.** The heads multiply only the predictor, which operates on
`dstc = 32`-dimensional latents with `hpre = 32`; the Impala encoder is
untouched and dominates cost. M=4 is therefore roughly 4x a *small* network, not
4x the model.

**Planning overhead.** Per iteration the cost pass runs
`[num_samples=200 x plan_length=90]` through the predictor; with M=4 tile to 800.
This is the dominant new cost and the reason the payoff experiment targets
compute rather than raw success: if uncertainty gating cannot beat 37 s/episode
at matched SR, the direction is not worth pursuing.

**Budget.** 3 seeds x (baseline, M=4 stock, M=4 split) = 9 runs, ~1.5-3 h each
at batch 64 -> roughly 15-25 T4-hours, inside Kaggle's weekly quota.

**Cut list, in order.** M=4 -> M=2; eval episodes 20 -> 10 (report the wider CI
honestly); epochs 12 -> 8; drop the `image_jepa`/`video_jepa` cross-checks.

---

## 8. Metrics

Raw success rate is not the headline: the baseline sits at 97 +/- 2 % with N=20
episodes over 3 seeds, so the interval is wide and the ceiling is close.

1. **Compute curve** (the headline): SR as a function of planning samples, for
   uncertainty-gated versus fixed-budget MPPI/CEM. Target statement: *equal
   success at X % less planning compute*, against the 37 s/episode baseline.
2. **Calibration**: NLL and ECE of predicted rollout error; Spearman between
   predicted uncertainty and realised error.
3. **Failure prediction**: AUROC for predicting episode failure from uncertainty
   *before* committing to the action sequence. This is where an uncertainty
   signal can win even when SR cannot move.
4. **The C1 falsifier**: Spearman(head disagreement, rollout error).
5. **Drift scaling**: measured bias versus M, against the `c^2 (1 - 1/M)`
   prediction.

Items 1 and 3 are the ones a reviewer can act on; item 4 is the one that can
kill the theory.

---

## 9. Risks and pre-registered falsifiers

| risk | kills | cheap test |
|---|---|---|
| Head disagreement is already informative | H1, the whole framing | phase 2, one run |
| Aleatoric term is dominated by encoder aliasing rather than action noise, so `g_hat` has no closed form | H3's clean cross-check | measure `g_hat` at two `img_size` settings |
| Encoder drift is small in practice, so C3 is vacuous | H4's relevance | measure target-encoder velocity over training |
| Two Rooms is too simple for a rich uncertainty signal | H5 | compare calibration on `video_jepa`/Moving MNIST |
| No compute headroom: MPPI is already near-optimal at 200 samples | H5 | SR versus samples at M=1, no new code |
| Uncertainty is not a valid risk signal under shift | H6 | OOD wall stress, pre-registered as likely negative |

The last row deserves emphasis: `Learning from World Feedback: Why Model
Uncertainty Fails as a Risk Signal in Model-Based RL` (arXiv:2607.16591,
2026-07) reports exactly this failure. H6 is written so that reproducing it
here is a *result*, not an embarrassment, and it connects the JEPA case to an
active debate.

---

## 10. Contribution path, naming, and how it stays honest

- This repo is independent; upstream keeps its name. Candidate names that
  describe the artefact rather than the umbrella: `jepa-uncertainty`,
  `epistemic-jepa`, `jepa-ensembles`, `ub-jepa` (*uncertainty-balanced*).
- Upstream surface is not crowded: 790 stars, 105 forks, 18 open issues, and
  **no** issue or discussion on uncertainty quantification or ensembling
  (checked 2026-09-28). Issue #31 (default configs assume ~24 GB, small-GPU
  guidance) is a natural ally: the reduced-batch recipe for T4 is itself a
  contribution, and it is what makes this work reproducible on modest hardware.
- Do not open a competing roadmap item. Land the example, then open one
  discussion referencing VJEPA (2601.14354) and UWM-JEPA (2605.25313).
- Upstream welcomes PRs but requires a CLA and 2-space/80-column style.
- The negative result is independently reportable. If H1 survives
  falsification, "JEPA predictor ensembles are not exchangeable, and their
  uncertainty estimates get worse with M" stands on its own without any planning
  payoff.

---

## 11. Related work (IDs verified via the arXiv API on 2026-09-28)

| work | id | relevance |
|---|---|---|
| EB-JEPA (upstream) | arXiv:2602.03604 | the codebase; states the goal of lowering the barrier to JEPA world-model research |
| PLDM / Two Rooms | arXiv:2502.14819 | the environment and its baseline |
| VJEPA | arXiv:2601.14354 | variational, uncertainty-aware JEPA; the closest prior framing |
| UWM-JEPA | arXiv:2605.25313 | belief-space latent prediction |
| Probabilistic JEPA as a hidden Markov model | arXiv (2026-08, ID not resolved -- API rate limit) | state-space reading of probabilistic JEPA |
| SIGReg as variational free energy | arXiv:2607.13612 | pressure on the objective / regularizer design |
| Beyond Gaussian Worlds | arXiv:2609.21656 | latent geometry versus Gaussian assumptions |
| MotionJEPA | arXiv:2609.23881 | temporal feature collapse, adjacent failure mode |
| FARM | arXiv:2609.11445 | failure signals from predictive states; closest to the failure-prediction metric |
| Why Model Uncertainty Fails as a Risk Signal | arXiv:2607.16591 | the pre-registered negative result |
| Where World Models Break | arXiv:2608.22421 | failure discovery under natural inputs |
| I Act Therefore I Am | arXiv:2609.31161 | identifiability of causal states in action-conditioned JEPA |
| Calibrated Value-Aware Model Learning (Voelcker et al.) | arXiv:2505.22772 | the variance-shrinkage pathology this work's C2 is the ensemble analogue of |

---

## 12. Immediate next actions

1. **Reproduce the baseline on the target hardware at reduced batch** (phase 1).
   Nothing else is meaningful until the 97 % baseline is reproduced inside the
   T4 memory envelope. This also produces `cfgs/train_ub_t4.yaml`, which is a
   standalone contribution.
2. **Run the C1 falsifier** (phase 2): M=4 stock objective, measure
   Spearman(head disagreement, rollout error). One run decides whether the
   project is what this document says it is.
3. **Verify the replicate construction against the dataset API** (before phase
   3): confirm `generate_actions` returns `bias_angle` in the training loop's
   batch, and that re-drawing two noise realisations reproduces the two
   observed next-states within tolerance.