"""Local trace viewer.

Reads run directories produced by the kernel (`events.jsonl`) and serves four
screens: run list, timeline, decision debugger, and comparison table. No login
and no database. The same files are what a later hosted history view would show.
"""

from __future__ import annotations

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlparse


def load_runs(root: Path) -> list[dict[str, Any]]:
    runs_dir = root / "runs" if (root / "runs").is_dir() else root
    found: list[dict[str, Any]] = []
    if not runs_dir.is_dir():
        return found
    for events in sorted(runs_dir.glob("*/events.jsonl")):
        summary = _summarize(events)
        if summary:
            found.append(summary)
    return found


def _summarize(path: Path) -> dict[str, Any] | None:
    events = _read_events(path)
    if not events:
        return None
    first = events[0]
    last = events[-1]
    payload = _payload(first)
    state = last.get("state") or {}
    usage = last.get("usage") or {}
    switches = sum(1 for ev in events if ev.get("kind") == "harness_switch")
    decisions = [ev for ev in events if ev.get("kind") == "decision"]
    backends = sorted({(ev.get("decision") or {}).get("backend", "") for ev in decisions if (ev.get("decision") or {}).get("backend")})
    return {
        "id": path.parent.name,
        "task": payload.get("task") or state.get("task_id") or "",
        "policy": payload.get("policy") or "",
        "harness": payload.get("harness") or state.get("harness_id") or "",
        "status": str(last.get("status") or ""),
        "usd": float(usage.get("usd") or 0),
        "actions": int(usage.get("actions") or 0),
        "switches": switches,
        "backends": backends,
        "events": len(events),
        "path": str(path.parent),
    }


def _read_events(path: Path) -> list[dict[str, Any]]:
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return rows


def _payload(event: dict[str, Any]) -> dict[str, Any]:
    raw = event.get("json_payload") or "{}"
    if isinstance(raw, dict):
        return raw
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {}


def run_detail(run_dir: Path) -> dict[str, Any]:
    events = _read_events(run_dir / "events.jsonl")
    steps = []
    for ev in events:
        kind = ev.get("kind")
        if kind not in {"decision", "action", "observation", "harness_switch", "verify", "decision_rejected", "decision_fallback", "run_finished"}:
            continue
        decision = ev.get("decision") or {}
        payload = _payload(ev)
        steps.append({
            "seq": ev.get("seq"),
            "kind": kind,
            "status": ev.get("status") or "",
            "chosen": decision.get("chosen_id") or "",
            "backend": decision.get("backend") or "",
            "rationale": decision.get("rationale") or "",
            "candidates": payload.get("candidates") or [],
            "dropped": payload.get("dropped") or {},
            "payload": {k: v for k, v in payload.items() if k not in {"candidates"}},
            "usd": (ev.get("usage") or {}).get("usd"),
            "harness": (ev.get("state") or {}).get("harness_id") or "",
            "failed": (ev.get("state") or {}).get("failed"),
        })
    return {"id": run_dir.name, "steps": steps}


def comparison_text(root: Path) -> str:
    path = root / "comparison.md" if root.is_dir() else Path()
    if path.is_file():
        return path.read_text(encoding="utf-8")
    return ""


class Viewer:
    def __init__(self, root: Path):
        self.root = root

    def serve(self, host: str, port: int) -> None:
        viewer = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                parsed = urlparse(self.path)
                if parsed.path in {"/", "/index.html"}:
                    body = _PAGE.encode("utf-8")
                    self._send(200, "text/html; charset=utf-8", body)
                    return
                if parsed.path == "/api/runs":
                    self._json({"runs": load_runs(viewer.root), "comparison": comparison_text(viewer.root)})
                    return
                if parsed.path == "/api/run":
                    run_id = parse_qs(parsed.query).get("id", [""])[0]
                    target = (viewer.root / "runs" / run_id)
                    if not (target / "events.jsonl").is_file():
                        target = viewer.root / run_id
                    if not (target / "events.jsonl").is_file():
                        self._send(404, "application/json", b'{"error":"missing run"}')
                        return
                    self._json(run_detail(target))
                    return
                self._send(404, "text/plain", b"not found")

            def _json(self, data: dict[str, Any]) -> None:
                self._send(200, "application/json", json.dumps(data).encode("utf-8"))

            def _send(self, code: int, ctype: str, body: bytes) -> None:
                self.send_response(code)
                self.send_header("Content-Type", ctype)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, fmt: str, *args: Any) -> None:
                return

        httpd = ThreadingHTTPServer((host, port), Handler)
        print(f"veyra view  http://{host}:{port}   runs: {self.root}")
        httpd.serve_forever()


_PAGE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<title>Veyra runs</title>
<style>
  :root { color-scheme: light; --ink:#1c1915; --muted:#5c564c; --line:#e4dfd6; --bg:#f7f4ee; --card:#fff; --accent:#8a4b08; }
  body { margin:0; font:15px/1.45 "Segoe UI", sans-serif; color:var(--ink); background:var(--bg); }
  header { padding:18px 24px; border-bottom:1px solid var(--line); background:var(--card); }
  h1 { margin:0; font-size:20px; }
  p.sub { margin:4px 0 0; color:var(--muted); }
  nav button { margin-right:8px; margin-top:10px; }
  main { padding:18px 24px 48px; display:grid; gap:16px; }
  button, select { font:inherit; padding:6px 10px; border:1px solid var(--line); background:var(--card); border-radius:6px; cursor:pointer; }
  button.on { border-color:var(--accent); color:var(--accent); }
  table { width:100%; border-collapse:collapse; background:var(--card); }
  th, td { text-align:left; padding:8px 10px; border-bottom:1px solid var(--line); vertical-align:top; }
  th { font-size:12px; letter-spacing:.04em; text-transform:uppercase; color:var(--muted); }
  .card { background:var(--card); border:1px solid var(--line); border-radius:10px; padding:12px 14px; }
  pre { white-space:pre-wrap; font:13px/1.4 ui-monospace, monospace; }
  .kind { font-weight:600; }
  .drop { color:#8a2b12; }
  .caveat { color:var(--muted); font-size:13px; }
</style>
</head>
<body>
<header>
  <h1>Veyra</h1>
  <p class="sub">Local trace viewer. Numbers on the comparison screen include the caveats stored with the run.</p>
  <nav>
    <button id="tab-runs" class="on">Runs</button>
    <button id="tab-trace">Trace</button>
    <button id="tab-decision">Decision</button>
    <button id="tab-compare">Comparison</button>
  </nav>
</header>
<main>
  <section id="view-runs"></section>
  <section id="view-trace" hidden></section>
  <section id="view-decision" hidden></section>
  <section id="view-compare" hidden></section>
</main>
<script>
const state = { runs: [], comparison: "", current: null, detail: null };
const tabs = ["runs","trace","decision","compare"];
tabs.forEach(name => {
  document.getElementById("tab-"+name).onclick = () => show(name);
});
function show(name) {
  tabs.forEach(n => {
    document.getElementById("view-"+n).hidden = n !== name;
    document.getElementById("tab-"+n).classList.toggle("on", n === name);
  });
}
function money(n) { return "$" + Number(n || 0).toFixed(4); }
async function boot() {
  const res = await fetch("/api/runs");
  const data = await res.json();
  state.runs = data.runs || [];
  state.comparison = data.comparison || "";
  renderRuns();
  renderComparison();
}
function renderRuns() {
  const rows = state.runs.map(r => `<tr data-id="${r.id}" style="cursor:pointer">
    <td>${r.task}</td><td>${r.policy}</td><td>${r.harness}</td><td>${r.status}</td>
    <td>${money(r.usd)}</td><td>${r.switches}</td><td>${(r.backends||[]).join(", ")}</td></tr>`).join("");
  document.getElementById("view-runs").innerHTML = `<div class="card"><table>
    <thead><tr><th>Task</th><th>Policy</th><th>Start</th><th>Status</th><th>Charged</th><th>Switches</th><th>Backend</th></tr></thead>
    <tbody>${rows || "<tr><td colspan=7>No runs in this directory.</td></tr>"}</tbody></table></div>`;
  document.querySelectorAll("#view-runs tr[data-id]").forEach(tr => {
    tr.onclick = () => openRun(tr.dataset.id);
  });
}
async function openRun(id) {
  state.current = id;
  const res = await fetch("/api/run?id=" + encodeURIComponent(id));
  state.detail = await res.json();
  renderTrace();
  renderDecision();
  show("trace");
}
function renderTrace() {
  const steps = (state.detail && state.detail.steps) || [];
  const body = steps.map(s => `<tr><td>${s.seq}</td><td class="kind">${s.kind}</td><td>${s.harness||""}</td><td>${s.chosen||""}</td><td>${s.backend||""}</td><td>${money(s.usd)}</td></tr>`).join("");
  document.getElementById("view-trace").innerHTML = `<div class="card"><h2>${state.current}</h2><table>
    <thead><tr><th>#</th><th>Event</th><th>Harness</th><th>Chosen</th><th>Backend</th><th>Ledger</th></tr></thead>
    <tbody>${body}</tbody></table></div>`;
}
function renderDecision() {
  const steps = ((state.detail && state.detail.steps) || []).filter(s => s.kind === "decision" || s.kind === "decision_rejected");
  const blocks = steps.map(s => {
    const cands = (s.candidates || []).map(c => `<li>${c.id} · success ${c.expected_success} · cost ${c.est_cost_usd} · risk ${c.risk}</li>`).join("");
    const dropped = Object.entries(s.dropped || {}).map(([id, why]) => `<li class="drop">${id}: ${why}</li>`).join("");
    return `<div class="card"><div class="kind">step ${s.seq} · ${s.chosen || s.kind} · ${s.backend}</div>
      <p>${s.rationale || ""}</p>
      <strong>Allowed</strong><ul>${cands || "<li>none recorded</li>"}</ul>
      <strong>Dropped by policy</strong><ul>${dropped || "<li>none</li>"}</ul></div>`;
  }).join("");
  document.getElementById("view-decision").innerHTML = blocks || `<div class="card">Open a run from the list.</div>`;
}
function renderComparison() {
  const text = state.comparison || "No comparison.md next to these runs.";
  document.getElementById("view-compare").innerHTML = `<div class="card"><pre>${text.replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}</pre>
    <p class="caveat">The comparison file is the source of the headline numbers. Read its caveats before quoting a cost cut.</p></div>`;
}
boot();
</script>
</body>
</html>
"""
