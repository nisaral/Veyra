# Veyra

**A switch controller, handoff compiler, and measurement kit for agent harnesses.**

Veyra does not replace Claude Code, Codex, OpenHands, Terminus-2, or Omnigent. Those are harnesses. Veyra decides **whether to stay, switch, retry, verify, ask, or stop**, what **state** the next harness is allowed to see, and whether that choice **paid for itself**.

The policy is the only component that may act. A typed decision model (Kev) **scores** candidate items. It never writes the next message and never calls a tool. If the controller errors, times out, or is uncertain, the current harness continues (**fail-open**).

```bash
cd veyra
pip install -e "./python[dev]"
cd go && go build -o ../dist/veyra.exe ./cmd/veyra && cd ..
veyra doctor
veyra compare --spawn --split dev --out out/dev
veyra dashboard --runs out/dev
```

No public Harbor or Terminal-Bench number exists yet. Do not quote `out/kev/` or the 30-task suite as one.

## Why this exists

Same model, different harness, large score gaps (Harbor: e.g. GPT-5.4 on SWE-smith 3.1% vs 22.1%). Switching can look useful because **a second attempt** is useful. Veyra’s job is to measure **net headroom**: the extra success you get from using a *different* harness after subtracting the extra success you get from sampling the *same* harness again — **and** to do it cheaper, with an audit log.

## What ships today (v0.2)

| Piece | Role |
|---|---|
| Go kernel | Loop, budget, fail-open fallback, append-only `events.jsonl` |
| Python sidecar | Harnesses, Kev client, MCPAgentBench **smoke** adapter |
| `HandoffV1` | [`veyra/schemas/handoff.v1.json`](veyra/schemas/handoff.v1.json) |
| Dashboard | Local replay of recorded runs (`veyra dashboard`) |

Reference harnesses (`native`, `repair`, MiniGraph) are for checkpoint/resume tests. Reviewers should treat **mini-swe-agent** (already an adapter) as the first external arm. Omnigent / Claude Code / Codex CLI are the wrap targets, not things to reimplement.

## Evaluation (Harbor first)

Spend **$0** on a notebook over Harbor’s public 8×2×54×3 release before any paid run. Then:

1. **Terminal-Bench 2.0** (89 tasks) — Harbor-native, mid-strength model, 20-task cost pilot, then 3 harnesses × 3 seeds.
2. **One SWE-style Harbor adapter** (~100 tasks, split by **repository**).
3. **Aider Polyglot** — cheap, large published gaps.
4. **Harbor-Index** — 82 hard tasks, rescue candidates.
5. **STT-Arena** — shift and abstain (later; simulated tools).

MCPAgentBench is a **smoke test**. ToolSandbox, τ-bench, TUA-Bench, SWE-bench come after Gate 1.

**Gate 1 (stop rule):** 95% CI upper bound on *net* headroom &lt; 2pp **and** no cost win at non-inferior success → publish the null, do not train a router.

Full spec: [`veyra/docs/OBJECTIVE.md`](veyra/docs/OBJECTIVE.md), [`veyra/docs/BENCHMARKS.md`](veyra/docs/BENCHMARKS.md). Tag `v0.2.0-pre-mcpagentbench` is the freeze before this plan.

## Resume — what to say about this project

Use these bullets as written. Do not inflate them with unpublished scores.

- Built a **Go execution kernel** with a portable state contract, budget ledger, and a policy that is the only actor; decision models cannot invent actions.
- Defined **HandoffV1** (failed actions, open subgoals, verification) as a versioned schema so a switch is a compiler problem, not a prompt dump.
- Wired an open **System One** decision model (Kev) as a **selector**, with embedding-pruning as the bar — not as a generative summarizer.
- Pre-registered a **two-axis gate**: net oracle headroom vs same-harness resampling, plus cost at non-inferior success, with a CI-based stop rule.
- Shipped a **local dashboard** that replays traces (candidates, policy drops, ledger) for audit without hosting execution.
- Positioned the work as an **adapter over Harbor agents** (mini-SWE-agent, Terminus-2, later Claude Code / Codex), complementary to Omnigent rather than a second meta-runtime.

**Do not put on a resume:** “beats ReAct on MCPAgentBench”, “learned policy”, “SWE-bench SOTA”, or the scripted −37.9% / Kev 80% tables.

## License

Apache-2.0. See [`veyra/LICENSE`](veyra/LICENSE).
