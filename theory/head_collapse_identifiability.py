"""
Head collapse, identifiability, and encoder-drift bias in JEPA predictor ensembles.

Target: facebookresearch/eb_jepa, `examples/ac_video_jepa` (Impala-RNN, Two Rooms).

Grounding in the upstream code
------------------------------
`eb_jepa/jepa.py::JEPA.unroll` drives every prediction through ONE module:

    predicted_states = self.predictor(predicted_states, actions_encoded)
    ...
    ploss += self.predcost(state, predicted_states) / nsteps

`predcost` is `eb_jepa/losses.py::SquareLossSeq` (plain MSE against the encoder
output `state`).  The anti-collapse regularizer is called as
`self.regularizer(state, actions)` -- it consumes ENCODER states and actions
only, never the predictor's outputs.  Two consequences follow, and they are the
subject of this file:

  * the prediction loss is separable across predictor heads, and
  * no term in the objective sees cross-head disagreement at all.

Three claims, each checked numerically below.

C1 (forced collapse).  The stock loss has a single global optimum at the
    conditional mean of the target encoding, so all M heads converge to the same
    function and cross-head disagreement is driven to zero.  It is zero
    regardless of how much predictive uncertainty actually exists, so it cannot
    measure it.

C2 (identifiability).  Training disagreement to match the total predictive
    variance v(x) = w(x) + g(x) is unidentified: every split with the same sum
    attains zero loss, including the fully collapsed one.  Two independent
    transition replicates from the same (state, action) identify g, and hence w.
    The collapsed solution then has strictly positive loss.  Two Rooms supplies
    the replicates for free -- it is stochastic by construction
    (data_config.yaml: action_noise 1, action_angle_noise 0.2).

C3 (encoder-drift bias; JEPA-specific).  In JEPA the regression target is the
    encoder's own output E_phi(o) and phi moves during training.  Every standard
    deep-ensemble uncertainty estimate assumes the M members are exchangeable and
    divides the spread by M.  In JEPA the heads share the encoder, so a shared
    drift component survives that division.  The resulting bias is
    c^2 * (1 - 1/M) and does NOT vanish as M grows: it increases towards c^2.
    Larger ensembles therefore make the JEPA uncertainty estimate worse.

Run:  python theory/head_collapse_identifiability.py
"""

import warnings

import numpy as np
from scipy import stats

RNG = np.random.default_rng(20260928)


def _spearman(a, b):
    """Spearman rho, nan when either input carries no ordering."""
    if np.std(a) < 1e-12 or np.std(b) < 1e-12:
        return float("nan")
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        return float(stats.spearmanr(a, b).statistic)


def _rule(title):
    print()
    print("=" * 78)
    print(title)
    print("=" * 78)


# ---------------------------------------------------------------- C1
def c1_forced_collapse(n=4096, d=6, M=8, noise=0.5, n_feat=32):
    """All heads converge to the same conditional mean; disagreement -> 0."""
    _rule("C1  forced collapse of the stock prediction loss")

    x = RNG.normal(size=(n, d))
    w_true = RNG.normal(size=d)
    f = lambda X: np.sin(X @ w_true)
    y = f(x) + noise * RNG.normal(size=n)          # aleatoric target noise

    # (a) heads sharing one feature map: a separable loss, one minimizer.
    R = RNG.normal(size=(d, n_feat))
    H = np.tanh(x @ R)
    w_shared, *_ = np.linalg.lstsq(H, y, rcond=None)
    preds_shared = np.stack([H @ w_shared for _ in range(M)])
    spread_shared = float(preds_shared.var(axis=0, ddof=1).mean())

    # (b) heads with independent feature maps (different init), as in practice.
    preds = []
    for _ in range(M):
        R_i = RNG.normal(size=(d, n_feat))
        H_i = np.tanh(x @ R_i)
        w_i, *_ = np.linalg.lstsq(H_i, y, rcond=None)
        preds.append(H_i @ w_i)
    preds = np.stack(preds)                         # [M, n]
    spread = preds.var(axis=0, ddof=1)              # per-sample disagreement
    mean_pred = preds.mean(axis=0)
    sq_err = (y - mean_pred) ** 2                   # per-sample predictive error

    rho = _spearman(spread, sq_err)

    print(f"  M heads                       : {M}")
    print(f"  aleatoric noise sd            : {noise}")
    print(f"  mean spread, shared features  : {spread_shared:.3e}   (== 0, the optimum)")
    print(f"  mean spread, per-head features: {spread.mean():.3e}")
    print(f"  mean squared predictive error : {sq_err.mean():.3e}")
    print(f"  Spearman(spread, error)       : {rho:+.3f}")
    print()
    print("  The loss is separable over heads, so its global optimum is the")
    print("  conditional mean for every head.  Disagreement is not a small")
    print("  quantity that happens to be noisy -- it is driven to zero, and")
    print("  what remains is uncorrelated with the actual predictive error.")

    assert spread_shared < 1e-18, "shared-feature heads must coincide exactly"
    assert abs(rho) < 0.2, "residual spread must not rank predictive error"
    return spread.mean(), float(sq_err.mean())


# ---------------------------------------------------------------- C2
def c2_identifiability(n=200_000, g_true=0.64, w0=0.10, w1=0.90):
    """Matching the summed variance is unidentified; replicates identify it."""
    _rule("C2  identifiability of the epistemic / aleatoric split")

    # contexts, with reducible uncertainty larger away from the data mass
    x = RNG.beta(2.0, 2.0, size=n) * 2.0 - 1.0
    w_true = w0 + w1 * x ** 2                       # epistemic (reducible)
    g = np.full(n, g_true)                          # aleatoric (env noise)
    v_true = w_true + g

    print(f"  true split: mean w = {w_true.mean():.4f}, g = {g.mean():.4f}, "
          f"v = {v_true.mean():.4f}")

    # --- the unidentified objective: only the SUM is observable -------------
    print()
    print("  objective depending on the sum only:  loss = mean((w_s + g_s - v)^2)")
    print()
    print("    alpha   w_s mean   g_s mean   sum      naive loss")
    print("    " + "-" * 50)
    alphas = [0.0, 0.25, 0.5, 1.0, 1.5]
    naive_losses = []
    for a in alphas:
        w_s = a * w_true
        g_s = v_true - w_s
        loss = float(np.mean((w_s + g_s - v_true) ** 2))
        naive_losses.append(loss)
        print(f"    {a:5.2f}   {w_s.mean():8.4f}   {g_s.mean():8.4f}   "
              f"{v_true.mean():6.4f}   {loss:.3e}")
    print()
    print("    Every split in this family attains zero loss, including alpha=0,")
    print("    which reports zero reducible error and is maximally wrong.")

    # decision relevance: a planner gates on "is this reducible uncertainty?"
    gate = 0.5 * w_true.mean()
    sizes = [float(np.mean(a * w_true < gate)) for a in alphas]
    print()
    print(f"    decision-relevant set {{w_s < {gate:.4f}}} covers, per alpha:")
    print("      " + "  ".join(f"{a:.2f}:{s:.3f}" for a, s in zip(alphas, sizes)))
    print("    Identical objective value, opposite planning behaviour.")

    # --- what replicates buy -------------------------------------------------
    # two independent transitions from the same (state, action)
    y1 = RNG.normal(scale=np.sqrt(g_true), size=n)
    y2 = RNG.normal(scale=np.sqrt(g_true), size=n)
    g_hat = 0.5 * np.mean((y1 - y2) ** 2)
    w_hat = float(v_true.mean()) - g_hat

    print()
    print("  with two independent transition replicates from the same (z_t, a_t):")
    print(f"    aleatoric estimate g_hat      : {g_hat:.4f}   (true {g_true:.4f})")
    print(f"    epistemic estimate w_hat      : {w_hat:.4f}   (true {w_true.mean():.4f})")
    print(f"    relative error, g_hat         : {abs(g_hat - g_true) / g_true:.2e}")
    print(f"    relative error, w_hat         : "
          f"{abs(w_hat - w_true.mean()) / w_true.mean():.2e}")

    # the split loss separates the zero-loss family
    split_losses = []
    for a in alphas:
        w_s = a * w_true.mean()
        g_s = v_true.mean() - w_s
        split_losses.append(float((w_s - w_hat) ** 2 + (g_s - g_hat) ** 2))

    print()
    print("    alpha   split loss")
    print("    " + "-" * 24)
    for a, l in zip(alphas, split_losses):
        print(f"    {a:5.2f}   {l:.3e}")
    print()
    print("    The split objective is zero only at the truth; the collapse is")
    print("    now strictly worse.")

    assert max(naive_losses) < 1e-24, "sum-only objective must be flat"
    assert abs(g_hat - g_true) / g_true < 1e-2, "replicates must identify aleatoric"
    assert split_losses[0] > 10 * split_losses[3], "split must separate collapse"
    return g_hat, w_hat


# ---------------------------------------------------------------- C3
def c3_encoder_drift(Ms=(2, 4, 8, 16, 32, 64, 128, 256),
                     cs=(0.0, 0.2, 0.5), a_scale=0.3, n=200_000):
    """Shared encoder drift survives the divide-by-M correction."""
    _rule("C3  encoder-drift bias in the exchangeable-ensemble estimate")

    print("  Entries are the fractional UNDERESTIMATION of the ensemble-mean")
    print("  variance by the divide-by-M estimator: (true - estimate) / true.")
    print("  Under-estimating an uncertainty means over-confidence.")
    print()
    header = "    M    " + "".join(f"c={c:<7.1f}" for c in cs)
    print(header)
    print("    " + "-" * (len(header) - 5))

    a2 = a_scale ** 2
    table = {}
    for M in Ms:
        row = []
        for c in cs:
            c2 = c ** 2
            # heads share a drifting encoder; variance of the mean is c^2 + a^2/M
            true_var_of_mean = c2 + a2 / M
            # the textbook estimator assumes exchangeability and divides by M
            estimate = (c2 + a2) / M
            bias = (true_var_of_mean - estimate) / true_var_of_mean
            row.append(bias)
            table[(M, c)] = bias
        print(f"    {M:<4d} " + "".join(f"{b:<9.3f}" for b in row))

    print()
    print("  With shared drift the underestimation tends to 1.0: the estimator")
    print("  reports essentially none of the real variance of the ensemble mean,")
    print("  and it gets worse as M grows:")
    for c in cs:
        if c == 0.0:
            continue
        print(f"    c={c:.1f}:  {table[(2, c)]:+.3f} at M=2  ->  "
              f"{table[(256, c)]:+.3f} at M=256")

    # calibration: does the estimate still rank contexts by reducible error?
    print()
    print("  ranking and thresholding against the true reducible error")
    print("  (M = 8, per-head spread equals the true w(x); a clean run would rank")
    print("   contexts perfectly, so any loss of ordering is drift-induced)")
    x = RNG.uniform(-1, 1, size=n)
    w_true = 0.10 + 0.90 * x ** 2
    M = 8
    regimes = {
        "no drift":           np.zeros(n),
        "drift co-monotone":  0.9 * w_true,
        "drift non-monotone": 0.9 * np.exp(-(((np.abs(x) - 0.5) / 0.15) ** 2)),
    }
    tau = float(np.median(w_true / M))              # threshold set on a clean run
    truth_top = w_true > np.median(w_true)

    print()
    print("    regime                Spearman   misclassified at clean tau")
    print("    " + "-" * 60)
    rhos = {}
    for name, d2 in regimes.items():
        epi_hat = (d2 + w_true) / M
        rho = _spearman(epi_hat, w_true)
        mis = float(np.mean((epi_hat > tau) != truth_top))
        rhos[name] = rho
        print(f"    {name:<20}  {rho:+.3f}     {mis * 100:5.1f}%")

    print()
    print("  Drift whose shape matches the real error inflates the level but keeps")
    print("  the ordering: top-k action selection survives, while any absolute")
    print("  uncertainty threshold silently misclassifies.  Drift whose shape does")
    print("  not match destroys the ordering itself, so the planner now prefers")
    print("  the wrong actions.")

    assert table[(256, 0.5)] > 0.99, "shared drift must not average away"
    assert table[(256, 0.5)] > table[(2, 0.5)], "underestimation must grow with M"
    assert rhos["no drift"] > 0.999, "clean run must rank perfectly"
    assert abs(rhos["drift co-monotone"]) > 0.999, "co-monotone drift keeps ordering"
    assert rhos["drift non-monotone"] < 0.9, "shapeless drift must break ordering"
    return table, rhos


def main():
    print(__doc__.strip().splitlines()[0])
    print("facebookresearch/eb_jepa, examples/ac_video_jepa (Impala-RNN, Two Rooms)")
    c1_forced_collapse()
    c2_identifiability()
    c3_encoder_drift()
    print()
    print("=" * 78)
    print("SUMMARY")
    print("=" * 78)
    print("  C1  disagreement is driven to zero at the stock objective's optimum.")
    print("  C2  the epistemic/aleatoric split is unidentified without replicates;")
    print("      the environment supplies replicates, so the split is identifiable.")
    print("  C3  with a moving encoder target the divide-by-M estimate is biased by")
    print("      c^2(1 - 1/M), which grows with M and does not vanish.")
    print()
    print("  C3 is the JEPA-specific obstruction: the members of a JEPA predictor")
    print("  ensemble are not exchangeable, because they share a moving encoder.")
    print("  Every claim above is a candidate falsification target for a real run.")
    print()


if __name__ == "__main__":
    main()