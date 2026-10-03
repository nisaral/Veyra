"""`veyra-py`: the Python-side command surface.

    veyra-py serve                       # harness + decision plane
    veyra-py compare --spawn --split dev # headline benchmark, one command
    veyra-py tasks                       # list the bundled suite
    veyra-py train --runs-dir runs       # fit the bandit policy from traces
    veyra-py doctor                      # environment check
"""

from __future__ import annotations

import argparse
import json
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from veyra.bench import report, runner
from veyra.bench.report import write as write_report
from veyra.bench.tasks import all_tasks, by_split

DEFAULT_SIDECAR = "127.0.0.1:7777"
DEFAULT_KERNEL = "127.0.0.1:7788"


def _wait_port(addr: str, timeout: float = 20.0) -> bool:
    host, _, port = addr.rpartition(":")
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with socket.create_connection((host or "127.0.0.1", int(port)), timeout=1.0):
                return True
        except OSError:
            time.sleep(0.25)
    return False


def _find_kernel(explicit: str) -> str:
    if explicit:
        return explicit
    for candidate in ("dist/veyra.exe", "dist/veyra", "veyra.exe", "veyra", "go/veyra.exe"):
        if Path(candidate).exists():
            return candidate
    return ""


def cmd_serve(args: argparse.Namespace) -> int:
    from veyra.server import serve

    serve(args.addr, harness_opts={"workspace": args.workspace} if args.workspace else {},
          default_backend=args.decision,
          decision_opts={"endpoint": args.von_endpoint or None, "state_path": args.bandit_state or None})
    return 0


def cmd_tasks(args: argparse.Namespace) -> int:
    tasks = all_tasks() if args.split == "all" else by_split(args.split)
    for t in tasks:
        print(f"{t.id:20s} {t.category:13s} {t.split:5s} {t.instruction[:70]}")
    print(f"\n{len(tasks)} tasks")
    return 0


def _load_results(path: str) -> list[dict[str, Any]]:
    src = Path(path)
    if src.is_dir():
        src = src / "results.json"
    if not src.exists():
        return []
    data = json.loads(src.read_text(encoding="utf-8"))
    return list(data.get("results") or [])


def _caveats(results: list[dict[str, Any]], args: argparse.Namespace | None = None) -> list[str]:
    mode = getattr(args, "model_mode", "offline")
    first = (
        "model_mode=offline: the scripted model proves the kernel/harness/decision plumbing and the wire "
        "protocol, not model behaviour. Headline numbers require a live model (--model-mode ollama)."
        if mode == "offline"
        else f"model_mode={mode}: a single deterministic run per (task, arm) at temperature 0. No seed "
        "sweep, so variance across samples is unmeasured."
    )
    out = [
        first,
        "Harness cost is charged as a per-step overhead on top of measured model usage, so harness "
        "selection is an economic decision rather than a free one.",
    ]
    backends = {b for r in results for b in (r.get("backends") or [])}
    engines = {e for r in results for e in (r.get("harness_engines") or [])}
    if "von-surrogate" in backends:
        out.append("veyra:von ran as `von-surrogate` (no --von-endpoint): it is NOT a real Jev-style model "
                   "and must not be reported as one.")
    if "bandit-untrained" in backends:
        out.append("veyra:bandit ran untrained (no --bandit-state): it defers to the heuristic baseline "
                   "and is labelled `bandit-untrained` in the event log.")
    if "builtin-graph" in engines:
        out.append("the `langgraph` arm used the in-repo MiniGraph fallback (LangGraph unavailable in this "
                   "environment), reported as engine:builtin-graph.")
    return out


def cmd_compare(args: argparse.Namespace) -> int:
    tasks = all_tasks() if args.split == "all" else by_split(args.split)
    out = Path(args.out)
    dev_results: list[dict[str, Any]] = []
    if args.arms:
        wanted = {a.strip() for a in args.arms.split(",")}
        arms = [a for a in runner.default_arms() if a.name in wanted]
    elif args.select_from:
        dev_results = _load_results(args.select_from)
        best, _ = report.pick_best_fixed(dev_results)
        if not best:
            print(f"compare: no fixed:* arms found in {args.select_from}", file=sys.stderr)
            return 2
        arms = runner.test_arms(best)
        print(f"compare: dev selected fixed:{best}; held-out arms = {[a.name for a in arms]}")
    else:
        arms = runner.default_arms()

    sidecar = None
    kernel = None
    try:
        if args.spawn:
            kernel_bin = _find_kernel(args.kernel_bin)
            if not kernel_bin:
                print("compare: --spawn needs the Go kernel; build it with `make build` or pass --kernel-bin",
                      file=sys.stderr)
                return 2
            from veyra.server import build_server

            if _wait_port(args.sidecar, timeout=0.4):
                print(f"compare: something is already listening on {args.sidecar}; refusing to start a "
                      "second sidecar", file=sys.stderr)
                return 2
            decision_opts = {"endpoint": args.von_endpoint or None,
                             "state_path": args.bandit_state or None}
            server, _ = build_server(args.sidecar, {}, "heuristic", decision_opts)
            server.start()
            sidecar = server
            if not _wait_port(args.sidecar):
                print(f"compare: sidecar did not come up on {args.sidecar}", file=sys.stderr)
                return 2
            if _wait_port(args.addr, timeout=0.4):
                print(f"compare: something is already listening on {args.addr}; refusing to --spawn over "
                      "it (a stale kernel would silently serve old code). Stop it, or drop --spawn to "
                      "attach to it deliberately.", file=sys.stderr)
                return 2
            kernel = subprocess.Popen(
                [kernel_bin, "serve", "--addr", args.addr, "--python", args.sidecar,
                 "--runs-dir", str(out / "runs")],
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            )
            deadline = time.time() + 30
            while time.time() < deadline:
                if kernel.poll() is not None:
                    output = kernel.stdout.read() if kernel.stdout else ""
                    print("compare: kernel exited immediately (stale binary or port already held):\n" + output,
                          file=sys.stderr)
                    return 2
                if _wait_port(args.addr, timeout=0.5):
                    break
            else:
                print("compare: kernel did not come up", file=sys.stderr)
                return 2

        print(f"comparing {len(tasks)} tasks x {len(arms)} arms against {args.addr}")

        def progress(record: dict) -> None:
            if not args.quiet:
                flag = "ok " if record["status"] == "SUCCEEDED" else "   "
                print(f"  {flag}{record['task']:20s} {record['arm']:16s} {record['status']:16s} "
                      f"${record['usd']:.4f} {record['actions']:>3d} actions sw={record['switches']}", flush=True)

        started = time.time()
        model = {"mode": args.model_mode, "cheap": args.model_cheap, "strong": args.model_strong,
                 "base_url": args.base_url}
        results = runner.run_all(tasks, arms, args.addr, out / "workspaces", timeout_s=args.timeout,
                                 on_result=progress, model=model)
        info = runner.meta(args.split, len(tasks), args.model_mode, runner.model_detail(model))
        info["elapsed_s"] = round(time.time() - started, 1)
        info["caveats"] = _caveats(results, args)
        paths = write_report(results, info, out, dev_results=dev_results or None)
        print()
        print(write_report.__module__ and Path(paths["markdown"]).read_text(encoding="utf-8"))
        print(f"wrote {paths['markdown']}")
        if paths["png"]:
            print(f"wrote {paths['png']}")
        return 0
    finally:
        if kernel is not None:
            kernel.terminate()
            try:
                kernel.wait(timeout=5)
            except subprocess.TimeoutExpired:
                kernel.kill()
        if sidecar is not None:
            sidecar.stop(grace=1.0)


def cmd_train(args: argparse.Namespace) -> int:
    from veyra.decision.bandit import train_from_events

    runs = sorted(Path(args.runs_dir).glob("*/events.jsonl"))
    if not runs:
        print(f"train: no run logs under {args.runs_dir}", file=sys.stderr)
        return 1
    model = train_from_events([str(p) for p in runs], alpha=args.alpha)
    model.state_path = Path(args.out)
    model.save()
    print(f"trained {len(model.theta)} arms from {len(runs)} runs -> {args.out}")
    return 0


def cmd_view(args: argparse.Namespace) -> int:
    from veyra.view import Viewer

    Viewer(Path(args.runs)).serve(args.host, args.port)
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    from veyra.harness.registry import HarnessRegistry

    print(f"python:  {sys.version.split()[0]} at {sys.executable}")
    registry = HarnessRegistry()
    for row in registry.triage():
        mark = "OK " if row["available"] else "NO "
        print(f"{mark}{row['id']:10s} {row['detail']}")
    from veyra.decision import von

    ok, detail = von.available(args.von_endpoint or None)
    print(f"{'OK ' if ok else 'NO '}von        {detail}")
    ready = _wait_port(args.addr, timeout=1.5)
    print(f"{'OK ' if ready else 'NO '}kernel     {args.addr} {'reachable' if ready else 'not running'}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="veyra-py", description="Veyra Python sidecar and benchmark")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_serve = sub.add_parser("serve", help="run the harness + decision sidecar")
    p_serve.add_argument("--addr", default=DEFAULT_SIDECAR)
    p_serve.add_argument("--decision", default="heuristic")
    p_serve.add_argument("--workspace", default="")
    p_serve.add_argument("--von-endpoint", default="")
    p_serve.add_argument("--bandit-state", default="")
    p_serve.set_defaults(func=cmd_serve)

    p_tasks = sub.add_parser("tasks", help="list the bundled benchmark tasks")
    p_tasks.add_argument("--split", default="all", choices=["dev", "test", "all"])
    p_tasks.set_defaults(func=cmd_tasks)

    p_cmp = sub.add_parser("compare", help="run the harness-switching comparison")
    p_cmp.add_argument("--addr", default=DEFAULT_KERNEL)
    p_cmp.add_argument("--sidecar", default=DEFAULT_SIDECAR)
    p_cmp.add_argument("--split", default="dev", choices=["dev", "test", "all"])
    p_cmp.add_argument("--arms", default="")
    p_cmp.add_argument("--out", default="out/dev")
    p_cmp.add_argument("--timeout", type=float, default=180.0)
    p_cmp.add_argument("--spawn", action="store_true", help="start the sidecar and the Go kernel")
    p_cmp.add_argument("--kernel-bin", default="")
    p_cmp.add_argument("--quiet", action="store_true")
    p_cmp.add_argument("--select-from", default="",
                       help="dev results dir/file; restricts test arms to the dev-selected baseline")
    p_cmp.add_argument("--bandit-state", default="", help="trained LinUCB policy json")
    p_cmp.add_argument("--von-endpoint", default="", help="Jev-style typed decision model HTTP endpoint")
    p_cmp.add_argument("--model-mode", default="offline", choices=["offline", "ollama", "openai"],
                       help="offline = scripted, deterministic; ollama/openai = live model")
    p_cmp.add_argument("--model-cheap", default="", help="cheap tier model name for live modes")
    p_cmp.add_argument("--model-strong", default="", help="strong tier model name for live modes")
    p_cmp.add_argument("--base-url", default="", help="endpoint for live modes (default: provider default)")
    p_cmp.set_defaults(func=cmd_compare)

    p_train = sub.add_parser("train", help="fit the contextual bandit from recorded runs")
    p_train.add_argument("--runs-dir", default="out/dev/runs")
    p_train.add_argument("--out", default="policies/bandit.json")
    p_train.add_argument("--alpha", type=float, default=0.6)
    p_train.set_defaults(func=cmd_train)

    p_view = sub.add_parser("view", help="local trace viewer for a runs directory")
    p_view.add_argument("--runs", default="out/dev")
    p_view.add_argument("--host", default="127.0.0.1")
    p_view.add_argument("--port", type=int, default=7860)
    p_view.set_defaults(func=cmd_view)

    p_doc = sub.add_parser("doctor", help="environment check")
    p_doc.add_argument("--addr", default=DEFAULT_KERNEL)
    p_doc.add_argument("--von-endpoint", default="")
    p_doc.set_defaults(func=cmd_doctor)

    args = parser.parse_args(argv)
    return int(args.func(args) or 0)


if __name__ == "__main__":
    raise SystemExit(main())