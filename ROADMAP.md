# Halberd Roadmap

Next steps and architectural milestones for the Halberd MCP reverse-proxy firewall.

## v0.2 — Streaming & Enhanced I/O Guardrails

- **Server-Sent Events (SSE) stream inspection**: Real-time per-event inspection for streaming MCP transport.
- **T3 Out-of-Scope I/O network guard**: CIDR network allowlisting and path canonicalization to prevent unauthorized network and filesystem egress.
- **Interactive browser ruleset builder UI**: Visual policy bundle builder in the WebAssembly playground ([halberd-keep.vercel.app](https://halberd-keep.vercel.app)).
- **Multi-arg array matching**: Support array-valued arguments in the policy DSL (e.g. `read_multiple_files`).

## v1.0 — Ecosystem & Observability

- **Dynamic MCP schema validation**: Verify requested tool calls against live upstream MCP schemas during session initialization.
- **Automated policy generator**: CLI command (`halberd init <mcp-server>`) to scaffold starter YAML policies by introspecting tool definitions.
- **Prometheus & OpenTelemetry metrics**: Expose standard metrics for decision latency, block counts, and audit event traces.

## Out of Scope

Per [CONTRIBUTING.md](CONTRIBUTING.md), Halberd explicitly does not aim to:

- Inspect raw LLM prompt/completion text (handled at the model gateway layer).
- Perform static analysis of MCP server source code (use `mcp-scan`).
- Act as an identity provider or manage user authentication.

---

For technical specifications, see [`docs/FIREWALL_ARCHITECTURE.md`](docs/FIREWALL_ARCHITECTURE.md) and [`docs/policy-dsl.md`](docs/policy-dsl.md).
