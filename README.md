# Veyra

Adaptive agent runtime in [`veyra/`](veyra/). It keeps a portable execution state, a budget ledger, and a policy that can switch harness at a checkpoint. The decision model scores candidates. It does not act.

The harness that wins the bundled offline suite is **repair**: it loads the failing file before the model call and edits before it executes. On the held-out split it reaches 100% success at $0.0096 per task. The plain loop is 80% at $0.0078. The graph harness is 100% at $0.0190. Switching into repair ties repair's cost. It does not beat starting there. These numbers use the scripted model. A live model run is still required before treating them as agent quality.

```bash
cd veyra
pip install -e ./python
# build dist/veyra with Go, then:
python -m veyra.cli compare --spawn --split dev --out out/dev
python -m veyra.cli view --runs out/dev
```

`plan.txt` is a rejected superset of this runtime. Do not build the control plane, marketplace, or database stack from it.

## Uncertainty-Preserving JEPA Predictor Ensembles

Research project built on [`facebookresearch/eb_jepa`](https://github.com/facebookresearch/eb_jepa),
targeting `examples/ac_video_jepa` (Impala-RNN predictor, Two Rooms navigation).

**Status: pre-registration / planning.** No training run has been executed yet.
The theory artifact runs; every empirical claim below is still a hypothesis with
a stated falsification bar.

## The problem

The obvious way to get uncertainty out of a JEPA predictor is to train M >= 2
prediction heads over the shared encoder and read their disagreement. That does
not work, and the reason is structural rather than a matter of tuning:

- In `eb_jepa/jepa.py::JEPA.unroll` the prediction loss is **separable across
  heads** and every head is scored against the same target, so the only global
  optimum has all heads at the conditional mean. Disagreement is driven to zero.
- The anti-collapse regularizer is called as `self.regularizer(state, actions)`
  with `state = self.encoder(observations)`. It never sees the predictor's
  outputs, so **head-collapse incurs zero regularization penalty**.
- In JEPA the prediction target is the encoder's own output and the encoder
  moves during training, so the heads share a moving target. They are **not
  exchangeable**, which breaks the assumption behind the standard
  "divide the spread by M" uncertainty estimate.

## What this project claims

See `theory/head_collapse_identifiability.py` (runs on CPU in seconds,
numpy + scipy only):

| | claim | measured in the artifact |
|---|---|---|
| C1 | the stock objective forces disagreement to zero | spread `2.7e-33` at the optimum; Spearman with true error `+0.04` |
| C2 | the epistemic/aleatoric split is unidentified under a sum-only objective | a zero-loss family spanning 100% to 0% in planner-facing decisions; replicates recover `g` to 0.5% and separate the family by ~8100x |
| C3 | shared encoder drift makes the divide-by-M estimate inconsistent, and it gets *worse* with M | 99.5% underestimation at M=256 with drift `c=0.5`, versus exact and M-independent at `c=0` |

C3 is the JEPA-specific obstruction: it does not exist in the
teacher-student/ensemble-distillation setting where the sibling problem was
previously studied.

## Why Two Rooms can support the fix

The environment transition is deterministic
(`env.py::_generate_transition` is `location + action`), so the aleatoric term is
*not* environment stochasticity. It comes from action-sampling noise (Von Mises
angle noise, truncated-normal step) and encoder aliasing. Crucially,
`dot_dataset.py::generate_actions` returns the commanded direction `bias_angle`
alongside the realised actions, which supplies the replicate structure the
identifiability fix needs.

## Roadmap

| phase | what | gate |
|---|---|---|
| 0 | theory artifact | done, assertions pass |
| 1 | baseline reproduction at reduced batch (T4) | MPPI SR within 5 pts of 97% |
| 2 | C1 falsifier: disagreement vs rollout error | kills the framing if Spearman > 0.5 |
| 3 | replicate split training | calibration better than stock |
| 4 | frozen vs joint uncertainty branch | drift bias shrinks when frozen |
| 5 | calibration-gated planning compute | equal SR at lower cost |
| 6 | OOD wall/door stress | pre-registered as likely negative |

Full specification, compute budget and related work:
[`docs/RESEARCH-PLAN-2026-09-28.md`](docs/RESEARCH-PLAN-2026-09-28.md).

## Reproduce the theory artifact

    python theory/head_collapse_identifiability.py

Verified on Python 3.11.9, numpy 2.1.1, scipy 1.17.1. No GPU, no torch required.
Interpreter used:
`C:/Users/nisar/AppData/Local/Microsoft/WindowsApps/python3.11.exe`

## Upstream context

`facebookresearch/eb_jepa` is Apache-2.0, ~790 stars, with no existing issue or
discussion on uncertainty quantification or ensembling (checked 2026-09-28).
Upstream issue #31 records that the default example configs assume ~24 GB VRAM;
the reduced-batch configuration this project needs is itself a useful
contribution.
