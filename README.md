# Veyra — Verified Execution Semantics for AI-Callable Tools

> **Don't trust tool declarations or LLM guesses. Test what tools actually do under failure before letting agents run autonomously.**
>
> Veyra actively tests tool execution semantics under controlled fault injection, discovers contract violations, and generates machine-readable **Verified Execution Profiles** that runtimes can enforce.

---

## 1. The Core Problem

When AI agents invoke tools (APIs, databases, payment gateways, filesystem endpoints), tool metadata often advertises capabilities that fail under real network conditions:
- An API claims to be **idempotent**, but lacks deduplication across retried requests.
- A status probe reports **committed=False** due to read-replica lag, inducing catastrophic duplicate mutations on blind retry.
- An MCP tool provides `readOnlyHint` or `idempotentHint`, which the protocol explicitly defines as unverified hints.

Blindly trusting developer declarations or LLM guesses results in duplicate payments, double database insertions, and unrecoverable real-world harm.

---

## 2. The Solution: Conformance Testing First, Runtime Enforcement Second

Veyra bridges the gap between **declared** and **verified** tool semantics:

```text
Tool Declaration (OpenAPI / MCP Hint / YAML)
              │
              ▼
┌──────────────────────────────────────┐
│      Veyra Conformance Engine        │
│  Active Fault Injection & Testing    │
│  (Lost ACK, Lag, Late Commit, Crash) │
└──────────────────┬───────────────────┘
                   │
                   ▼
┌──────────────────────────────────────┐
│      Verified Execution Profile      │
│  (Proven Idempotency, Probes, Bounds)│
└──────────────────┬───────────────────┘
                   │
                   ▼
┌──────────────────────────────────────┐
│      Veyra Execution Runtime         │
│  Enforce Verified Boundaries         │
└──────────────────────────────────────┘
```

---

## 3. Installation & Development Setup

*(Note: The PyPI package name `veyra` is currently occupied by an unrelated project. Install from source or local checkout):*

```bash
git clone https://github.com/nisaral/Veyra.git
cd Veyra
pip install -e ./python
```

Verify environment:
```bash
veyra doctor
```

---

## 4. Auditing a Tool: Conformance Profile

Audit any tool contract or MCP manifest against execution failure modes:

```bash
veyra analyze-tool examples/execution_contracts/payment_charge.yaml
```

Run active black-box conformance testing:
```bash
veyra conformance run examples/execution_contracts/payment_charge.yaml
```

Inspect verified execution profile:
```bash
veyra conformance profile examples/execution_contracts/payment_charge.yaml
```

---

## 5. Provenance-Tracked Execution Contract

Every capability tracked by Veyra carries explicit provenance:
- `DECLARED`: Stated by developer or MCP ToolAnnotation hint.
- `OBSERVED`: Seen in runtime execution traces.
- `VERIFIED`: Actively proven by Veyra black-box fault-injection testing.
- `CONTRADICTED`: Claimed by documentation but violated under fault injection.

---

## 6. Research & Evaluation Status

- **Evaluation Protocol:** See [Partial Observability Protocol](docs/research/partial_observability_protocol.md) and [Fair Baseline Spec](docs/research/fair_baseline_spec.md).
- **Historical Experiments:** Versioned synthetic evaluation reports are archived in [docs/research/archive/](docs/research/archive/).
- **Current Runtime Status:** Production default is **Deterministic Contract Enforcement** backed by verified execution profiles. The Bayesian Belief-State controller is maintained as an **experimental research arm** for partial observability studies.

*Disclaimer: Veyra does NOT claim mathematically zero risk or "100% safety enforcement." All empirical metrics report finite-sample confidence bounds (e.g., 0 observed duplicate effects in N trials; one-sided 95% UCB = X%).*

---

## 7. License

Apache 2.0.
