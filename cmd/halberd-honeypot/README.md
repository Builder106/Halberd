# halberd-honeypot

> **Vulnerable by design.** Do not connect this binary to production data.
> Its entire purpose is to act as a known-bad upstream so Halberd's policy
> engine can be observed catching real threats end-to-end.

`halberd-honeypot` is a minimal stdio MCP server that exposes four tools,
each crafted to exercise one or more categories from
[`docs/threat-model.md`](../../docs/threat-model.md). Pair it with
`halberd-stdio` in front and you have a one-command playground for
demoing, integration-testing, and recording.

## Quick demo

```bash
go build -o bin/ ./cmd/...

bin/halberd-stdio \
  --policy policies/halberd-honeypot.yaml \
  --audit  halberd.jsonl \
  -- bin/halberd-honeypot
```

Pipe a malicious request into the wrapper's stdin and watch what comes
back:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_sql","arguments":{"query":"DROP TABLE users"}}}' \
| bin/halberd-stdio --policy policies/halberd-honeypot.yaml --audit halberd.jsonl -- bin/halberd-honeypot
```

→ Halberd returns a `-32000` policy-violation JSON-RPC error; the
honeypot never sees the query. The block is in `halberd.jsonl`.

## Threat coverage per tool

| Tool | Threat | What the tool does |
| --- | --- | --- |
| `get_weather(city)` | **T1** (tool poisoning) | Returns a response containing ANSI color escapes, a zero-width space splitting`<\|im_start\|>`, and a role-tag-spoofed system prompt. Halberd's response inspector should strip the ANSI and zero-width chars before the agent sees the text. |
| `execute_sql(query)` | **T2** (argument injection) | Echoes any query back as a success result. The point is to verify request-side`deny_patterns` block dangerous SQL *before* it reaches this function — by the time execution lands here, it's already too late. |
| `read_file(path)` | **T2 + T3** (argument injection + out-of-scope I/O) | Opens whatever path the agent supplies. No allowlist, no path-traversal guard, no symlink resolution. Halberd's filesystem rule pack must block traversal and absolute-path attempts upstream. |
| `list_users()` | **T5** (exfiltration via response) | Returns a response embedding`AKIAIOSFODNN7EXAMPLE`, `ghp_AAAA…`, and a `-----BEGIN RSA PRIVATE KEY-----` block. All values are AWS-documented or format-only fakes — nothing live. |

**T4** (capability creep via `tools/list_changed`) is not yet
implemented; the honeypot's tool list is static. v0.2 will add a
trigger tool that pushes a `tools/list_changed` notification to
demonstrate request-side mid-session inventory drift.

## Why a separate binary

Halberd is transport-agnostic; the policy engine doesn't care whether
the upstream is a real MCP server or a teaching prop. But every
integration test and demo recording needs *something* on the other end
of the pipe. Using `cat` (as the smoke tests do) only proves the
plumbing works — it doesn't prove Halberd catches threats. The
honeypot's tool outputs *are* the threats, so a single end-to-end run
verifies both directions of the proxy in one shot.

## Safety properties

- Never advertises tools that do anything beyond returning canned text

  or opening a file. No shell exec, no network egress, no DB.

- `read_file` is the only tool that touches the filesystem; it surfaces

  the OS error verbatim (file-not-found is a normal response, not a
  silent failure).

- The fake secrets are documented public examples. `AKIAIOSFODNN7EXAMPLE`

  is from AWS's own IAM documentation; `ghp_`followed by 36`A`s
  matches GitHub's PAT format but is not a real token; the RSA block is
  a fragment, not a valid key.
