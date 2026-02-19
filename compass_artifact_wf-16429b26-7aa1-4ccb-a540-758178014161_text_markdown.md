# Production multi-agent AI orchestration: solving the hard architectural problems

**The most reliable autonomous coding agents in 2025–2026 converge on a shared architectural philosophy: trust nothing the model claims, verify everything through external ground truth, and treat the harness—not the model—as the source of reliability.** This report synthesizes implementation patterns from Devin, OpenHands, SWE-agent, Claude Code, GitHub Copilot, Cursor, Factory AI, Codegen, Augment Code, and the major orchestration frameworks (LangGraph, Temporal, Inngest) to document how production systems solve six specific architectural challenges. The patterns emerging across these systems reveal a clear hierarchy: deterministic verification gates beat LLM-based evaluation, structured state artifacts beat conversational memory, and cost/iteration budgets remain the most reliable circuit breaker against runaway agents.

---

## 1. Effect verification: how systems prove agents actually did the work

The core problem is straightforward but critical: an LLM can claim it created files, fixed bugs, or passed tests without any of that being true. Production systems solve this through layered verification that separates agent claims from observed ground truth.

**Anthropic's harness pattern** (published November 2025 in "Effective Harnesses for Long-Running Agents") provides the canonical architecture. An initializer agent creates a structured `feature_list.json` file with every feature marked `"passes": false`. Coding agents may only flip a feature to `true` after testing it end-to-end. The system prompt uses strongly-worded instructions: *"It is unacceptable to remove or edit tests because this could lead to missing or buggy functionality."* JSON was chosen over Markdown because **the model is less likely to inappropriately change or overwrite JSON files**. A `claude-progress.txt` file persists across sessions as the canonical record of completion. Each session begins with a startup ritual: `pwd` → read progress file → read feature list → `git log --oneline -20` → start dev server → run smoke test. If the smoke test fails, the agent must fix existing bugs before starting new work.

**Git-diff-based evidence collection** is the most common verification mechanism across platforms. Anthropic's coding agents commit after each feature with descriptive messages; subsequent sessions read `git log` to reconstruct state. OpenAI's Codex runs each task in an isolated cloud sandbox, requires agents to run any programmatic checks specified in `AGENTS.md`, and provides **terminal log citations** (`T:chunk_id:line_start-line_end`) and **file path citations** (`F:file_path`) as verifiable audit trails. The `git-ai` project (December 2025) takes this further by recording pre/post-edit checkpoints as diffs in `.git/ai/`, creating an Authorship Log that links every line to the agent session that produced it—surviving rebases, merges, and cherry-picks.

**Ground truth verification follows a clear hierarchy** used across production systems:

- **Level 1 — Deterministic checks**: Compilation passes, test suite green, linting clean. Anthropic's "Demystifying Evals" notes that *"deterministic graders are natural for coding agents because software is generally straightforward to evaluate: Does the code run and do the tests pass?"*
- **Level 2 — Filesystem reconciliation**: Expected files exist with expected content. OpenHands snapshots files before changes for revert capability.
- **Level 3 — Git evidence**: `git diff` shows the expected changes were made.
- **Level 4 — End-to-end testing**: Anthropic's agents use Puppeteer MCP for browser automation, taking screenshots and navigating the app as a user would. This caught bugs invisible from code inspection alone (though it cannot see browser-native alert modals).
- **Level 5 — LLM-based evaluation**: Model graders with clear rubrics assess agent behavior from transcripts—least reliable, used as final layer.
- **Level 6 — Human review**: All production systems require human approval before merge.

**Platform-specific verification approaches** reveal meaningful differences. **GitHub Copilot's coding agent** runs a security validation pipeline before finalizing PRs: CodeQL for code security, the GitHub Advisory Database for malware and high-severity CVEs in new dependencies, and secret scanning for leaked credentials. **Devin** integrates a test-debug-fix cycle where the agent runs tests, analyzes failures, adds debugging statements, makes fixes, and re-runs—looping until all tests pass. With Sonnet 4.5 integration, Devin added proactive self-verification where the model fetches HTML of web pages to verify UI behavior. **OpenHands** uses a CodeAct architecture embedding LLM reasoning into a unified coding control plane that produces and validates intermediate artifacts. **SWE-agent** takes a deliberately minimal approach—verification is entirely external, running the project's existing test suite in a Dockerized harness.

**Cursor's multi-agent architecture** uses a **Judge agent pattern**: Planners explore the codebase and create tasks, Workers execute tasks and push changes, and Judge agents determine whether to continue at each cycle end. This separates the verification concern from the execution concern architecturally. **Augment Code** generates **8 candidate solutions per problem** in parallel, then uses an ensembler model to select the best one—a voting-based verification approach.

The key implementation insight across all systems is that **sandbox isolation is the foundation** of trustworthy verification. OpenAI Codex enforces OS-level sandboxing with network disabled by default, write permissions limited to the active workspace, and configurable modes (`read-only`, `workspace-write`, `danger-full-access`). Platforms like E2B, Daytona (sub-90ms sandbox creation), and Blaxel provide purpose-built agent sandboxes.

---

## 2. Detecting infinite loops and semantic stagnation

Explicit loop detection remains surprisingly rare in production systems. Only **OpenHands has a dedicated `StuckDetector` class**—most systems rely on budget limits and iteration caps as blunt but effective circuit breakers.

**OpenHands' `StuckDetector`** (found in `openhands/controller/stuck.py`) is the most sophisticated production implementation. It analyzes the full agent history to detect repeated patterns, checking for hardcoded syntax errors ("unterminated string literal", "invalid syntax", "incomplete input") appearing in loops. It returns a `StuckAnalysis` dataclass containing `loop_type`, `loop_repeat_times`, and `loop_start_idx`. In headless/automated mode, it analyzes all history; in interactive mode, only history after the last user message. The detector is called in the `_step()` method of `AgentController` between budget checks and agent execution. A known limitation: loop detection can incorrectly kill agents waiting on legitimate long-running processes (tracked as Issue #5355), prompting a feature request for an explicit `WaitAction(seconds: int)`.

**Claude Code lacks built-in loop detection** as of early 2026. GitHub Issue #4277 requests an "Agentic Loop Detection Service" similar to Google Gemini CLI's `loopDetectionService.ts`. Current safeguards include a `--max-turns` flag for non-interactive mode and context window compaction at ~92% usage. Claude Code uses **TODO lists as a focus mechanism**—Manus-style todo list rewriting keeps goals in the model's recent attention span, reducing "lost-in-the-middle" drift. Known failure mode: agents can get stuck "chanting" (generating the same phrase repeatedly), particularly in non-interactive/SDK scenarios.

**LangGraph provides framework-level recursion limits**, counting supersteps (node executions) with a default limit of **25**. A `RemainingSteps` pattern enables graceful degradation:

```python
def supervisor_node(state):
    if state["remaining_steps"] <= 2:
        return Command(goto=END)
```

**AgentCircuit** (2025) offers composable decorators for loop detection via **state hashing**—if the same input appears `limit` times, it raises a `LoopError`. Its components include `Fuse` (loop detection), `Sentinel` (Pydantic schema validation on every output), `Medic` (LLM-based auto-repair of bad outputs), `BudgetFuse` (dollar-based circuit breaker), and `TimeoutFuse` (time-based circuit breaker). These compose with LangGraph, CrewAI, and AutoGen.

**Token and cost budgets are the most reliable circuit breakers in practice.** OpenHands uses configurable `max_budget_per_task` (default $4 USD) and `MAX_ITERATIONS` (default 100, extended by 100 per user message). CrewAI defaults to `max_iter=20` and `max_retry_limit=2`. The `AgentBudget` library provides zero-code-change budget enforcement with nested budgets for child agents. **Portkey's AI Gateway** implements classic circuit breakers monitoring error thresholds and failure rates, automatically removing unhealthy providers from the routing pool during a cooldown period.

**Anthropic identified two semantic no-progress failure modes** that budget limits alone won't catch. First, **premature declaration of victory**: the agent sees some progress and declares done. Solved by a structured feature list where incomplete features remain visible. Second, **one-shotting/context exhaustion**: the agent tries too much at once and runs out of context mid-implementation. Solved by enforcing incremental, feature-by-feature work with commits between each feature.

The **session startup ritual** doubles as a progress fingerprint: read progress file + git log → start dev server → run smoke test → if smoke test fails, fix existing bugs before proceeding → pick highest-priority incomplete feature. This ensures each session begins with a ground-truth assessment of actual project state rather than relying on any agent's memory.

---

## 3. Terminal recovery, escalation, and knowing when to give up

Production systems converge on **3–5 retries** for transient failures before escalation, with exponential backoff using a coefficient of 2.0 and initial intervals of 1 second.

**Specific retry defaults from production systems:**

| System | Max Retries | Max Iterations | Notes |
|---|---|---|---|
| Temporal (activities) | Unlimited by default | — | Must set explicitly; production standard: 3–5 |
| CrewAI | `max_retry_limit=2` | `max_iter=20` | Per agent |
| Vercel AI SDK | `maxRetries=2` | — | Per LLM call |
| Restate | `maxRetryAttempts=3` | — | Typical config |
| Google ADK LoopAgent | — | `max_iterations=5` | Typical loop limit |
| OpenHands | LLM retry decorator | 100 default | +100 per user message |

**Exponential backoff follows a standard formula**: `delay = initialInterval × backoffCoefficient^attemptNumber`, with typical production values of **1s initial, 2.0 coefficient, 60–100s maximum interval, ±20% jitter**. Temporal's default retry policy uses 1s initial interval, 2.0 backoff, and 100× initial for maximum interval.

**The quarantine state pattern** isolates repeatedly failing tasks. Guard conditions for entering quarantine include: consecutive failures ≥ 3 with same error type, total cost exceeding budget, execution time exceeding maximum, or identical failure repeated ≥ 2 times on same input. IBM's **STRATUS pattern** (NeurIPS 2025) uses transactional-no-regression (TNR) where only reversible changes are allowed—agents have undo operators for every action, and write locks prevent concurrent execution. On failure, the agent performs undo-and-retry with a different approach, outperforming state-of-the-art by **150%+ on cloud engineering benchmarks**.

**Automatic reassignment follows a model fallback chain**: primary model → cheaper model → alternative provider → local model. LangGraph implements this with conditional error edges:

```python
def route_on_failure(state):
    if state["error_type"] == "model_unavailable":
        return "fallback_model_node"
    elif state["retry_count"] >= 3:
        return "reassign_agent_node"
    elif state["retry_count"] >= 5:
        return "human_escalation_node"
    return "retry_same_node"
```

**Human escalation triggers in production** include: confidence score dropping below threshold twice consecutively, 2–3 failed attempts at the same task, destructive/irreversible actions proposed (database deletion, production deployment), task cost exceeding budget, and agent acting outside permission boundaries. Escalation mechanisms span Slack messages with approve/reject buttons, PR comments, PagerDuty/Opsgenie alerts for critical incidents, and webhook callbacks that pause workflows awaiting HTTP responses.

**The "give up gracefully" pattern** returns partial results with structured metadata: completed steps, failed step, error summary, partial output, and a recommendation for manual intervention. All intermediate state is persisted for potential manual resume. The key principles: return whatever was accomplished, log the complete execution trace, notify appropriate channels, and clean up resources.

---

## 4. Reviewer agent choreography and enforcing review gates

Production review agents in 2025–2026 operate primarily as advisory tools, with **merge-blocking enforcement requiring explicit CI/CD configuration** rather than being the default.

**PR-Agent/Qodo Merge** implements a layered architecture: user interfaces → orchestration → specialized tools → platform abstraction. The `PRAgent` class dispatches slash commands (`/review`, `/describe`, `/improve`) to specialized tool classes. It fires automatically on every new PR via configurable `pr_commands = ["/describe", "/review", "/improve"]`. Enforcement works through **auto-labeling**: the review tool applies labels like "possible security issue" and "review effort [x/5]". A separate GitHub Action blocks merges when specific labels are present. The commercial Qodo Merge product offers direct **merge gating**—the ability to block merges programmatically based on review criteria.

**CodeRabbit** runs a hybrid pipeline-agentic architecture on Google Cloud Run with sandboxed microVMs. Each PR review spins up an isolated, ephemeral environment. Its **Pre-Merge Checks** (introduced 2025) provide formal quality gates with three enforcement modes: `off`, `warning` (non-blocking, the default), and `error` (blocks merges when paired with Request Changes Workflow). Custom checks are defined in natural language (≤1000 characters) and run in a read-only sandbox. When a check in `error` mode fails, CodeRabbit submits a GitHub "Request Changes" review, blocking merge per branch protection rules. Overrides are per-PR and tagged `[IGNORED]` for audit traceability.

**GitHub Copilot Code Review** reviews PRs in under 30 seconds and posts inline comments with one-click suggested changes. However, it **explicitly never blocks merges**—it always posts as a "Comment" review, never "Approve" or "Request Changes." This is a deliberate design choice. The Copilot coding agent enforces a different structural constraint: **the person who requested the agent cannot approve its own PR**, ensuring independent human review.

**Factory AI's Droid Review** focuses on substantive bugs (dead code, broken control flow, async/await mistakes, null dereferences, resource leaks, race conditions) and explicitly skips stylistic concerns. Notably, **it submits approvals when no issues are found**—actively approving clean PRs, unlike Copilot which never approves.

The **canonical multi-agent review patterns** that have emerged include:

- **Evaluator-optimizer loop** (Anthropic's pattern): Generator produces code → Critic evaluates → Refiner addresses feedback → loop until criteria met or iteration limit reached
- **Fan-out/fan-in review**: SecurityAuditor, StyleEnforcer, and PerformanceAnalyst run in parallel, then a PRSummarizer consolidates findings
- **Maker-checker** (Microsoft Azure): Orchestrator decomposes → delegates to coding agent → review agent validates (mandatory gate) → approved or looped back with feedback
- **QA-Checker agent** (from CodeAgent research, arxiv 2402.02172): A supervisory agent ensures all specialist agents' contributions address the initial review question—the QA-Checker is non-optional

**Making AI review a true blocking gate requires explicit configuration:** deploy the review tool as a GitHub Actions workflow, configure it to exit non-zero on critical issues, add the job name as a required status check in branch protection, and optionally layer human CODEOWNER review as defense-in-depth. The emerging best practice is a **dual-gate**: AI review as required status check (Gate 1) plus CODEOWNERS-based human review (Gate 2), with both required before merge.

---

## 5. Audit telemetry: separating attempted from confirmed actions

The core problem is that repeated permission checks can create audit logs indistinguishable from repeated successful actions, producing a misleading record. ISACA's 2025 analysis frames this as the "Black Box Audit Trail" problem: *"When a human agent commits fraud, there is a clear audit trail: User ID 123 clicked button X at time Y. When an AI agent takes an action, the log often just shows that the 'System' executed a command."*

**The solution requires three distinct event types in the log schema**: `permission_checked` (authorization evaluated), `action_attempted` (tool call initiated), and `effect_confirmed` (outcome verified on disk/in environment). Each event carries a unique interaction identifier for idempotency tracking—distinguishing retries of the same action from genuinely new actions. Events are correlated so that permission grants without corresponding confirmed actions trigger alerts.

**Langfuse** (open-source, YC W23) provides the most mature structured observability for agent actions, with hierarchical tracing using distinct observation types: `event`, `span`, `generation`, `agent`, `tool`, `chain`, `retriever`, `evaluator`, and `embedding`. Since v3.3.1, its agent graph visualization infers execution flow from observation timings and nesting, visualizing complex looping patterns. Each observation captures input/output, model parameters, token usage, latency, tool calls with results, and success/failure status.

**Claude Code's permission model provides an inherent audit separation.** It operates in three permission modes: Default (asks for edits and commands), Auto-accept edits (asks only for commands), and Plan mode (read-only). Before every file edit, it snapshots current contents as a checkpoint. The key architectural distinction: *"Actions that affect remote systems (databases, APIs, deployments) can't be checkpointed, which is why Claude asks before running commands with external side effects."* Hooks provide monitoring and observability instrumented around tool use, errors, and decisions—enabling detection of drift, unsafe behavior, and performance regressions.

**OpenAI Codex creates a citation-based audit trail** where every claim the agent makes is backed by terminal log citations (with chunk IDs and line numbers) and file path citations. The `codex exec --json` command outputs newline-delimited JSON events per state change, and OpenTelemetry integration provides opt-in monitoring for compliance auditing.

**The three-layer defense architecture** recommended across production systems consists of: scoped permissions (agents get tokens limiting exact functions needed), step-up authentication (dynamic challenges triggered for sensitive workflows), and human-in-the-loop gating (AI prepares action, human approves execution). This naturally separates the telemetry into authorization events, execution events, and outcome events.

---

## 6. State machine design for autonomous agent orchestration

Production systems in 2025–2026 converge on **graph-based state machines** with durable execution, using three dominant infrastructure choices: LangGraph for application-layer orchestration, Temporal for enterprise-grade durability, and Inngest for serverless-first workflows.

**The canonical lifecycle state machine** follows this progression:

```
IDLE → ASSIGNED → WORKING → VERIFYING → REVIEWING → COMPLETED
                     ↓                       ↓
                  RETRYING ──────→ QUARANTINED → ESCALATED
                     ↑                              ↓
                     └──────────── REASSIGNED ←─────┘
```

Guard conditions on transitions enforce correctness: `attempt_count < MAX_RETRIES` gates retry, `schema_valid(output)` gates advancement to review, `confidence_score >= 0.85` determines auto-approve versus human review, and `execution_time < timeout` prevents runaway execution.

**LangGraph** implements state machines as `StateGraph` objects with typed state schemas, conditional edges for routing, and **checkpointing for crash recovery**. State is reducer-driven using Python `TypedDict` and `Annotated` types. Independent nodes execute in parallel, and the graph compiles to an immutable structure before execution. Recovery is encoded directly in the graph—nodes branch to error edges, trigger compensating actions, or roll back to the last checkpoint.

**Temporal provides the gold standard for durable execution.** Workflows are deterministic orchestration code persisted via an append-only event history. Activities (LLM calls, API calls) can fail and retry independently of the workflow. The default retry policy uses **1s initial interval, 2.0 backoff coefficient, 100s maximum interval, and unlimited attempts**—production systems override this to 3–5 attempts with explicit `NonRetryableErrorTypes`. Temporal workflows can run indefinitely (days, months) and resume from the exact point of failure. Signal-based human-in-the-loop patterns allow workflows to wait for human approval. A Temporal + OpenAI Agents SDK integration entered public preview in September 2025.

**Inngest** uses serverless step functions where each step is an atomic unit automatically retried and persisted. Its **AgentKit** provides multi-agent network orchestration with deterministic routing. Key differentiator: failure in step N doesn't restart steps 1 through N-1. Built-in flow control includes throttling, batching, prioritization, and concurrency limits.

**Anthropic's "Building Effective Agents" framework** (December 2024) defines five composable workflow patterns that serve as the building blocks for state machines: **prompt chaining** (sequential steps with validation gates), **routing** (classification-based dispatch), **parallelization** (sectioning or voting), **orchestrator-workers** (dynamic task decomposition), and **evaluator-optimizer** (generation with feedback loops). Their key recommendation: *"find the simplest solution possible, and only increase complexity when needed."* The Claude Agent SDK supports subagents with isolated context windows for parallelization, and **compaction** for long-running single-agent tasks—summarizing context at ~92% window usage.

**OpenHands provides the most granular state management visible in open source.** Its `AgentState` enum includes RUNNING, PAUSED, STOPPED, FINISHED, ERROR, and WAITING_FOR_CONFIRMATION. The controller `_step()` method follows a strict sequence: Delegation Check → Budget/Iteration Checks → State Checks → Loop Detection → Agent Execution → Security Analysis → Action Publishing → Metrics Update. Delegate agents get separate state but share event streams and metrics.

**JustCopy.ai's persistent state machine** offers a practical production case study. It tracks phases (Requirements → User Flows → Data Models → Frontend → Backend → Integration → Testing → Deployment) with per-phase state including progress percentage, interruption count, sandbox status, completed phases, and specific next steps. On resume after a crash, the agent receives this structured context enabling exact resumption—no replaying prior work.

---

## Conclusion: the harness is the product

The most important insight from this research is that **reliability in multi-agent systems comes from the harness, not the model**. Every production system studied—from Anthropic's own Claude Agent SDK to Devin to OpenHands—achieves reliability through external verification layers, deterministic checkpoints, and structured state management rather than trusting model outputs. The five key patterns that distinguish production systems from prototypes are: (1) structured JSON-based progress tracking that resists model manipulation, (2) git-based evidence chains providing immutable proof of work, (3) budget and iteration limits as the first line of defense against runaway agents, (4) durable execution infrastructure (Temporal, Inngest, or LangGraph checkpointing) enabling crash recovery without replaying work, and (5) mandatory human gates at merge time, regardless of how much autonomy the agent has during execution.

The notable gap is in **semantic loop detection**: only OpenHands has a dedicated `StuckDetector`, while Claude Code has an open feature request for one. Most systems rely on crude iteration limits. As agents take on longer, multi-day tasks, this area will need significant advancement. Similarly, **reviewer agent enforcement defaults to advisory** across every platform studied—achieving true merge-blocking AI review requires explicit CI/CD configuration that most teams haven't yet implemented. The dual-gate pattern (AI review + human CODEOWNER) represents the emerging best practice but remains more aspiration than standard deployment.