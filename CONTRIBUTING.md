# Contributing to Halberd

Thanks for the interest. Halberd is small and opinionated; this document
exists so we don't waste each other's time on PRs that won't land.

## Dev setup

```bash
brew install go         # or your platform's equivalent
git clone https://github.com/Builder106/halberd
cd halberd
go mod tidy
go test ./...
```

To run the HTTP proxy against a remote MCP server:

```bash
go build -o bin/halberd-http ./cmd/halberd-http
./bin/halberd-http --policy policies/mcp-server-postgres.yaml \
                   --target  http://localhost:8080 \
                   --listen  :9090
```

To wrap a local stdio MCP server (Claude Desktop / Cursor style):

```bash
go build -o bin/halberd-stdio ./cmd/halberd-stdio
./bin/halberd-stdio --policy policies/mcp-server-postgres.yaml \
                    --audit  halberd-stdio.jsonl \
                    -- mcp-server-postgres --conn-string "$PG_CONN"
```

To run the full v0.1 surface against the deliberately-vulnerable
honeypot (the demo target — never point at production data):

```bash
go build -o bin/ ./cmd/...
./bin/halberd-stdio --policy policies/halberd-honeypot.yaml \
                    --audit  halberd.jsonl \
                    -- ./bin/halberd-honeypot
```

See [`cmd/halberd-honeypot/README.md`](cmd/halberd-honeypot/README.md)
for the threat-coverage table.

## Project-specific guardrails

- **Latency is a feature.** The policy engine sits on every JSON-RPC request.

  p50 added latency must stay below 200 µs and p99 below 1 ms on the bench
  corpus. PRs that regress `BenchmarkEngine_*` are reverted.

- **No third-party dependencies without a strong reason.** Halberd currently

  depends on the Go standard library and `gopkg.in/yaml.v3`. Adding a
  dependency requires a paragraph in the PR description explaining why a
  stdlib equivalent does not work.

- **Rule packs are data, not code.** New protections for a specific MCP

  server go in `policies/<server-name>.yaml`, not in `internal/policy/`.

- **The engine is IO-free.** `internal/policy`does not import`os`, `net`,

  or `io`. Anything that talks to the outside world goes in `cmd/` or
  `internal/transport/`or`internal/audit/`.

## Commit and PR conventions

- Imperative-mood subject line ≤ 72 chars.
- Body explains *why*. The diff already explains *what*.
- One logical change per PR. If a refactor and a feature land together, split

  them — refactor first.

- All tests pass and `go vet ./...` is clean before opening the PR.

## Out of scope (please don't open these PRs)

- **WebSocket transport.** Deprecated in the 2025-06 MCP spec; Halberd only

  supports stdio and streamable HTTP+SSE.

- **TLS termination / mTLS.** Offload to a reverse proxy in front of Halberd

  (nginx, Caddy, Envoy).

- **A web UI / dashboard.** The audit log is JSONL on disk; pipe it into

  whatever you already use.

- **Real-time LLM-based response classification.** Latency budget kills it.

  Future v0.2 may add an async out-of-band classifier; in-band is not on the
  table.

- **Distributed clustering / replication.** Halberd is a single-process

  proxy. Run one per upstream MCP server.

## Cutting a release

Releases are tag-driven. Pushing a `v*` tag triggers
[`.github/workflows/release.yml`](.github/workflows/release.yml), which
runs goreleaser against the tag and publishes a GitHub Release with
multi-arch archives.

```bash

# 1. Make sure main is green and the JOURNAL has a release entry

gh run list --limit 1
$EDITOR JOURNAL.md

# 2. Dry-run goreleaser locally — catches config or build errors before

#    they fail in CI

goreleaser release --snapshot --clean --skip=announce,publish

# 3. Tag and push. The release workflow takes over from here

git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0

# 4. Watch the run; the release lands at

#    https://github.com/Builder106/halberd/releases/tag/v0.1.0

gh run watch
```

Goreleaser injects the tag value as `main.version` at link time, so
`halberd version`and the`halberd-honeypot` startup banner both report
the actual release version. Snapshot builds report `<next>-next-<sha>`.

## Reporting security issues

Do **not** open a public GitHub issue for a vulnerability in Halberd itself.
Email <vaughanolayinka@gmail.com> with the subject prefix `[halberd security]`.
