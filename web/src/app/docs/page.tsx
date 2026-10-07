"use client";

import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { BookOpen, Terminal, Code2, ShieldCheck, FileCheck, Layers, GitFork, RefreshCw, AlertOctagon, Cpu, CheckCircle2 } from "lucide-react";

export default function DocsPage() {
  return (
    <div className="min-h-screen flex flex-col bg-[#fcfcfc] text-slate-900">
      <Navbar />

      <main className="flex-grow py-12 sm:py-16">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
            {/* Left Docs Sidebar */}
            <aside className="lg:col-span-3 space-y-4">
              <div className="sticky top-24 technical-card p-4 bg-white">
                <div className="text-xs font-mono font-semibold text-slate-400 uppercase tracking-wider mb-3">
                  Veyra v0.2.0 Docs Index
                </div>
                <nav className="space-y-1 text-xs font-mono">
                  <a href="#quickstart" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-800 font-medium">1. Quickstart & Install</a>
                  <a href="#public-api" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">2. Unified Public API</a>
                  <a href="#frameworks" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">3. Framework Integrations</a>
                  <a href="#mcp-proxy" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">4. Veyra MCP Proxy</a>
                  <a href="#config-system" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">5. Configuration System</a>
                  <a href="#replay-diff" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">6. Replay & Trajectory Diffing</a>
                  <a href="#unknown-ack" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">7. UNKNOWN_ACK & Transactions</a>
                  <a href="#plugins" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">8. Plugin Architecture</a>
                  <a href="#cli-reference" className="block py-1.5 px-2 rounded hover:bg-slate-100 text-slate-600">9. CLI Reference</a>
                </nav>
              </div>
            </aside>

            {/* Main Docs Content */}
            <div className="lg:col-span-9 space-y-10">
              {/* Section 1: Quickstart */}
              <section id="quickstart" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-emerald-600 font-semibold">
                  <Terminal className="w-4 h-4" /> 1. Getting Started
                </div>
                <h2 className="text-2xl font-bold tracking-tight text-slate-900">
                  Quickstart & Installation
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  Veyra is execution control middleware for tool-using AI agents. It sits post-proposal, pre-execution to validate contracts, enforce policy, manage transaction state, and prevent unsafe side effects.
                </p>

                <div className="space-y-3 font-mono text-xs">
                  <div className="bg-slate-950 text-slate-100 p-4 rounded-lg border border-slate-800 space-y-2">
                    <div className="text-slate-400 text-[10px] uppercase">Install Veyra Package</div>
                    <pre className="text-emerald-400"><code>pip install veyra</code></pre>
                  </div>

                  <div className="bg-slate-950 text-slate-100 p-4 rounded-lg border border-slate-800 space-y-2">
                    <div className="text-slate-400 text-[10px] uppercase">Verify Environment</div>
                    <pre className="text-emerald-400"><code>veyra doctor</code></pre>
                  </div>
                </div>
              </section>

              {/* Section 2: Public API */}
              <section id="public-api" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <Code2 className="w-4 h-4 text-slate-700" /> 2. Unified Public API
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  One Clean Entry Point (`from veyra import Veyra`)
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  Wrap functions, MCP clients, HTTP clients, or agent instances with Veyra execution controls:
                </p>
                <div className="bg-slate-950 text-slate-100 p-4 rounded-lg font-mono text-xs border border-slate-800 leading-relaxed">
                  <pre>{`from veyra import Veyra

veyra = Veyra(policy="default", mode="fail_closed")

# Wrap a standard Python tool
@veyra.wrap
def process_payment(customer_id: str, amount: float) -> dict:
    return {"status": "success", "tx_id": "tx_9921"}

# Agent proposes tool call -> Veyra validates & executes
result = process_payment(customer_id="cust_102", amount=99.0)`}</pre>
                </div>
              </section>

              {/* Section 3: Framework Integrations */}
              <section id="frameworks" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <Cpu className="w-4 h-4 text-slate-700" /> 3. Framework Integrations
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  First-Party Framework Adapters
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  Veyra integrates seamlessly into major AI agent orchestration frameworks without modifying their graph or agent runtime:
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                  <div className="p-4 bg-slate-50 border border-slate-200 rounded-lg space-y-2">
                    <h3 className="font-mono font-semibold text-xs text-slate-900">OpenAI Agents SDK</h3>
                    <pre className="text-[11px] font-mono text-slate-700 bg-white p-2.5 rounded border border-slate-200">{`from veyra.integrations.openai_agents import VeyraOpenAIAgentsAdapter

adapter = VeyraOpenAIAgentsAdapter(veyra)
wrapped_tool = adapter.wrap_tool(my_tool)`}</pre>
                  </div>

                  <div className="p-4 bg-slate-50 border border-slate-200 rounded-lg space-y-2">
                    <h3 className="font-mono font-semibold text-xs text-slate-900">LangGraph</h3>
                    <pre className="text-[11px] font-mono text-slate-700 bg-white p-2.5 rounded border border-slate-200">{`from veyra.integrations.langgraph import VeyraLangGraphNode

node = VeyraLangGraphNode(veyra, [tool_a, tool_b])
graph.add_node("veyra_executor", node)`}</pre>
                  </div>

                  <div className="p-4 bg-slate-50 border border-slate-200 rounded-lg space-y-2">
                    <h3 className="font-mono font-semibold text-xs text-slate-900">AutoGen</h3>
                    <pre className="text-[11px] font-mono text-slate-700 bg-white p-2.5 rounded border border-slate-200">{`from veyra.integrations.autogen import VeyraAutoGenAdapter

adapter = VeyraAutoGenAdapter(veyra)
tool = adapter.register_function(fn)`}</pre>
                  </div>

                  <div className="p-4 bg-slate-50 border border-slate-200 rounded-lg space-y-2">
                    <h3 className="font-mono font-semibold text-xs text-slate-900">Microsoft Agent Framework</h3>
                    <pre className="text-[11px] font-mono text-slate-700 bg-white p-2.5 rounded border border-slate-200">{`from veyra.integrations.microsoft_agent_framework import VeyraMicrosoftAgentMiddleware

mw = VeyraMicrosoftAgentMiddleware(veyra)`}</pre>
                  </div>
                </div>
              </section>

              {/* Section 4: MCP Proxy */}
              <section id="mcp-proxy" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <Layers className="w-4 h-4 text-slate-700" /> 4. Veyra MCP Proxy
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  Zero-Code-Change Model Context Protocol Proxy
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  Run Veyra as an independent execution proxy between standard MCP clients and MCP servers:
                </p>
                <div className="bg-slate-950 text-slate-100 p-4 rounded-lg font-mono text-xs border border-slate-800 leading-relaxed">
                  <pre>{`$ veyra proxy mcp

# Architecture:
# Agent (MCP Client) -> Veyra MCP Proxy -> Veyra Policy/Resolution -> Real MCP Server`}</pre>
                </div>
              </section>

              {/* Section 5: Configuration System */}
              <section id="config-system" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <FileCheck className="w-4 h-4 text-slate-700" /> 5. Configuration System
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  Human-Readable `veyra.yaml` Config
                </h2>
                <div className="bg-slate-950 text-slate-100 p-4 rounded-lg font-mono text-xs border border-slate-800 leading-relaxed">
                  <pre>{`veyra:
  mode: fail_closed

policy:
  tenant_isolation: true
  authorization: true
  freshness: true
  health: true

recovery:
  unknown_ack: verify_then_defer
  retries:
    enabled: true
    max_attempts: 2`}</pre>
                </div>
                <div className="flex gap-4 font-mono text-xs pt-2">
                  <div className="p-3 bg-slate-100 rounded border border-slate-200 flex-1">
                    <span className="font-semibold text-slate-900">$ veyra config validate</span>
                    <p className="text-slate-600 text-[11px] pt-1">Validates mode, constraints, and retry limits.</p>
                  </div>
                  <div className="p-3 bg-slate-100 rounded border border-slate-200 flex-1">
                    <span className="font-semibold text-slate-900">$ veyra config explain</span>
                    <p className="text-slate-600 text-[11px] pt-1">Explains active policies and runtime rules.</p>
                  </div>
                </div>
              </section>

              {/* Section 6: Replay & Trajectory Diffing */}
              <section id="replay-diff" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <RefreshCw className="w-4 h-4 text-slate-700" /> 6. Replay & Trajectory Diffing
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  Dry-Run Trajectory Reconstruction
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  Reconstruct trajectory decisions without performing side effects by default (DRY_RUN mode):
                </p>
                <div className="bg-slate-950 text-slate-100 p-4 rounded-lg font-mono text-xs border border-slate-800 leading-relaxed">
                  <pre>{`# Dry-run replay of a trace JSON
$ veyra replay run.json

# Opt-in to live execution
$ veyra replay run.json --execute

# Compare step-by-step diffs between two runs
$ veyra diff run1.json run2.json`}</pre>
                </div>
              </section>

              {/* Section 7: UNKNOWN_ACK */}
              <section id="unknown-ack" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <ShieldCheck className="w-4 h-4 text-slate-700" /> 7. UNKNOWN_ACK & Transaction Engine
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  First-Class Non-Idempotent Mutation Protection
                </h2>
                <p className="text-xs sm:text-sm text-slate-600 leading-relaxed">
                  When network drops occur during non-idempotent mutations, Veyra classifies the outcome as <code>UNKNOWN_ACK</code>. Rather than blindly replaying the request (which risks duplicate charging or state corruption), Veyra enforces verification before replay or defers execution safely.
                </p>
              </section>

              {/* Section 8: Plugins */}
              <section id="plugins" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <GitFork className="w-4 h-4 text-slate-700" /> 8. Plugin Architecture
                </div>
                <h2 className="text-xl font-bold text-slate-900">
                  `@veyra_plugin` Decorator & Metadata Registry
                </h2>
                <div className="bg-slate-950 text-slate-100 p-4 rounded-lg font-mono text-xs border border-slate-800 leading-relaxed">
                  <pre>{`from veyra.plugins import veyra_plugin

@veyra_plugin(name="stripe_policy", version="1.0.0", capabilities=["payment_safety"])
class StripePolicyPack:
    ...`}</pre>
                </div>
              </section>

              {/* Section 9: CLI Reference */}
              <section id="cli-reference" className="technical-card p-6 sm:p-8 bg-white space-y-4">
                <div className="flex items-center gap-2 text-xs font-mono text-slate-500 font-semibold">
                  <Terminal className="w-4 h-4 text-slate-700" /> 9. CLI Command Reference
                </div>
                <div className="border border-slate-200 rounded-lg overflow-hidden text-xs font-mono">
                  <table className="w-full text-left">
                    <thead className="bg-slate-100 border-b border-slate-200 font-semibold text-slate-700">
                      <tr>
                        <th className="p-2.5">Command</th>
                        <th className="p-2.5">Description</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      <tr><td className="p-2.5 text-emerald-600 font-bold">veyra doctor</td><td className="p-2.5">Check Python environment and dependency readiness</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra init</td><td className="p-2.5">Initialize a default `veyra.yaml` config file</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra config validate</td><td className="p-2.5">Validate syntax and constraints of configuration</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra config explain</td><td className="p-2.5">Print human-readable breakdown of active policies</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra inspect tool &lt;name&gt;</td><td className="p-2.5">Inspect tool schema, parameters, and risk level</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra proxy mcp</td><td className="p-2.5">Start Veyra MCP proxy server (stdio/streamable HTTP)</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra replay &lt;file&gt;</td><td className="p-2.5">Replay execution trajectory trace in DRY_RUN mode</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra diff &lt;run1&gt; &lt;run2&gt;</td><td className="p-2.5">Diff tool selection, outcomes, and latencies between runs</td></tr>
                      <tr><td className="p-2.5 text-slate-900 font-medium">veyra bench run &lt;suite&gt;</td><td className="p-2.5">Execute reproducible benchmarks (e.g. undobench, mcpmark)</td></tr>
                    </tbody>
                  </table>
                </div>
              </section>
            </div>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  );
}
