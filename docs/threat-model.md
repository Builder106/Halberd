# Threat model

Halberd defends the boundary between an LLM agent and the MCP servers it
calls. Five threats are in scope; the v0.1 release covers two of them
end-to-end and lays the groundwork for the rest.

## T1 — Tool poisoning *(covered, v0.1)*

**Vector.** A compromised or adversarial MCP server returns content in a
`tools/call` result that hijacks the agent's next turn. Common payloads:
role-tag spoofing (`<|im_start|>system: ignore previous instructions`),
ANSI escape sequences that hide injected text from log readers,
zero-width-Unicode payloads that survive copy-paste.

**Halberd's job.** Inspect every chunk of every response, strip ANSI and
hidden Unicode, optionally rewrite role-tag markers, and block responses
that match injection-marker patterns.

## T2 — Argument injection *(covered, v0.1)*

**Vector.** The agent is induced — by a prior tool response, a poisoned
webpage in context, or a direct user instruction — to call a legitimate
tool with hostile arguments. `execute_sql("DROP TABLE users")`,
`git_clone("--upload-pack=ssh user@host")`, `shell_exec("rm -rf /")`.

**Halberd's job.** Match each argument against a per-tool rule:
`deny_patterns`(regex denylist),`allow_values` (strict enum),
`max_length`, `type`. Block on any match.

## T3 — Out-of-scope I/O *(planned, P5)*

**Vector.** A tool with an apparently-narrow purpose (e.g. "read project
files") is invoked with arguments that escape its sandbox: a path
traversal, a private-IP URL, an out-of-bound database query.

**Halberd's job.** Per-tool path / URL allowlists keyed on the tool's
declared purpose. Reject reads outside the project root, HTTP destinations
outside the declared origin set, SQL touching schemas not on the allowlist.

## T4 — Capability creep *(covered, v0.1)*

**Vector.** Mid-session, an MCP server pushes a `tools/list_changed`
notification advertising a new tool. The agent calls it before any human
or policy can review. The classic example: a "calculator" server that
silently adds an `exec_shell` tool an hour into the session.

**Halberd's job.** Pin the tool inventory at session start to whatever is
declared in the policy bundle. Any `tools/call` for a tool name not in the
bundle is rejected with category `T4_capability_creep` (v0.1 covers this
on the request side; the v0.2 roadmap adds a `tools/list_changed`
notification-side guard).

## T5 — Exfiltration via response *(covered, v0.1)*

**Vector.** A compromised tool's response contains secrets bound for the
model context: AWS access keys, GitHub tokens, RSA private key blocks,
session cookies. The model "helpfully" relays them to the user, an
attacker-controlled webhook, or pastes them into the next tool call.

**Halberd's job.** Scan response bodies with built-in detectors for the
common secret formats and rewrite them to `[REDACTED]` before they reach
the agent. Detectors live in `internal/policy/sanitize.go`; v0.1 ships
`aws_access_key`, `github_token`, and `rsa_private_key`. Operators
opt in per-bundle via `response_filters.global.secret_scanners`.

## Out-of-threat-model

Halberd does **not** defend against:

- Compromise of the agent process itself (memory-resident prompt injection,

  malicious system prompts). That's an LLM-layer concern.

- Compromise of the user's session credentials. That's the IdP's job.
- Side-channel timing attacks on the proxy itself. Latency variance from

  policy evaluation is bounded by the perf gates in CI; if you need
  constant-time policy lookup, file an issue.

- Denial of service from the upstream MCP server. Halberd assumes the

  upstream is honest-but-curious; for byzantine upstreams, run Halberd in
  front of a rate limiter and an upstream-health probe.
