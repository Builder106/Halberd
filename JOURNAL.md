# JOURNAL — Halberd

> Dated log of decisions, pivots, incidents, and quotes. Add entries as
> things happen — retrospectives need this raw material to land.
> Reverse-chronological; one paragraph max per entry.

## 2026-08-07 — Firewall architecture specification and feature audit #milestone

Executed a feature audit across the Go reverse proxy (`cmd/halberd-http`, `cmd/halberd-stdio`), policy engine (`internal/policy`), and WebAssembly bridge (`cmd/halberd-wasm`). Created `docs/FIREWALL_ARCHITECTURE.md` documenting threat mitigations T1 through T5, wax-seal verdict semantics (Refused, Pass Granted, Amended), and the client-side WASM engine architecture. Scheduled T3 out-of-scope I/O network guard enhancements and interactive browser ruleset builder UI for development.

## 2026-05-27 — Web type refinement: Manrope body + JetBrains Mono + parchment grain #decision

A type pass on the keep-tour brand, deliberately not a redesign. The previous build defined a ceremonial display-serif (Cormorant Garamond) but left the body sans and mono as system-font fallbacks — Manrope + JetBrains Mono now load via `next/font/google` so the brand carries through the body copy, the pipeline glyphs (`tools/call → policy → audit → upstream`), and every `<code>` block. Brass palette grew three stops: `--color-brass-lit` for candlelit hovers, `--color-brass-weathered` for aged seals/wordmarks, `--color-candle` (#f5c97a) for hearth-glow accents. The body now carries a fixed SVG parchment-grain layer warmed toward the brass channel plus a bottom-left candlelight wash, so the dark backdrop reads as torchlit stone instead of flat slate. Cormorant gained italic style (the "Every request must pass the gate." tagline was already italic; now it actually renders that way). **Why:** the Roman-numeral keep tour, wax-seal verdicts, and Cormorant headings were already strong — the body type was the only piece still reading as system default. **How to apply:** all new components should consume `var(--font-sans)` / `var(--font-mono)` / `var(--font-serif)` from globals; do not hardcode font stacks. The `.parchment-grain` and `.rule-brass` utility classes are opt-in for components that want them.

## 2026-05-27 — WASM-bridge contract suite (Playwright QA) #milestone #decision

The Gherkin demo suite tests the playground UI *through* the bridge; it won't fail on a schema change if the UI still happens to render. This new QA layer tests the bridge in isolation — loads `/` in headless Chromium, waits for `halberd-wasm` to register on `globalThis.halberd`, then calls `packs()` / `evaluateRequest()` / `evaluateResponse()` directly and asserts the JSON shapes. That's the layer that catches drift between `internal/policy` and `cmd/halberd-wasm`, which the Go tests can't (they don't compile to WASM). Eight tests in `web/e2e/qa/specs/wasm-bridge.spec.ts`: five packs returned, DROP TABLE blocked + safe SELECT allowed under postgres, AWS/GitHub/RSA redaction under the honeypot, error envelope for an unknown pack, malformed JSON-RPC handled without crashing, multi-byte unicode round-trips clean, version string advertised. Config split per the baseline — `playwright.qa.config.ts` is parallel / headless / no video / retries-on-flake, the exact inverse of the demo config. `npm run test:qa` boots `next start` via Playwright's webServer hook; CI's `web` job runs it after `next build`. 25s end-to-end with the build folded in.

## 2026-05-27 — Vercel kept showing the default triangle: it scans /favicon.ico, not the SVG #incident

The dashboard project card kept rendering Vercel's own triangle even though `public/favicon.svg` was wired into `metadata.icons`. Cause: the create-next-app scaffold's `favicon.ico` was still sitting in `src/app/`, served at `/favicon.ico` by App Router's auto-link convention, and Vercel's project-card avatar scanner pulls `/favicon.ico` specifically — it doesn't read the SVG. Rendered a multi-res (16/32/48/64) `.ico` from `assets/favicon.svg` via `rsvg-convert` + `magick` and dropped it in. Modern browsers still prefer the SVG linked alongside in `layout.tsx`; the `.ico` is only there for clients that won't honor SVG and for Vercel's scanner. Lesson: a metadata-linked SVG favicon isn't enough — anything that hardcodes the `/favicon.ico` path (Vercel's scanner here) needs the real file present.

## 2026-05-27 — @vercel/analytics + @vercel/speed-insights wired in #milestone

Both packages installed into `web/` and mounted as client components
in the root layout. Build clean, deploy green (`c33c02c`).

State as of this commit:

- **Speed Insights**: fully live. `/_vercel/speed-insights/script.js`
  serves; the SpeedInsights component fires Web Vitals beacons on
  every visit. Dashboard at `vercel.com/sankofa-forge/halberd/speed-
  insights` will populate as traffic arrives.
- **Web Analytics**: code wired, receiving endpoint still 404s. The
  project record has `webAnalytics.id` populated but on the Hobby
  plan the `/_vercel/insights/script.js` endpoint only flips live
  after a manual "Enable Web Analytics" click in the dashboard.
  The API endpoint for programmatic enable
  (`POST /v1/web/insights/projects/{id}/enable`) returns 404 on
  Hobby — only paid plans get programmatic activation.

To finish the install: visit
[vercel.com/sankofa-forge/halberd/analytics](https://vercel.com/sankofa-forge/halberd/analytics)
and click *Enable Web Analytics*. One-time, no code change needed.

## 2026-05-27 — Vercel git-trigger deploys were failing silently for 11h #incident

Every git push since `010f1f0` (~11 hours ago) was triggering a
Vercel deploy that errored out in 4-6s with: `Couldn't find any
pages or app directory. Please create one under the project root`.
The live site stayed up only because none of those errored deploys
got promoted, so the prior CLI-issued deploy stuck as production.
Caught when I checked the Vercel dashboard — the deploys list was
a column of red.

Root cause: when I ran `vercel link --project halberd` and
`vercel deploy --prod` from `web/`, the CLI link wrote
`.vercel/project.json` pointing at the right project but the
**project's `rootDirectory` setting on Vercel's side stayed at
the repo root**. The GitHub integration (which Vercel set up when
the project was created) honors that server-side setting, not the
local `.vercel/project.json`. So:

- CLI deploys from `web/` → succeed (build runs in `web/`)
- Git push → fail (build runs in repo root, no Next.js there)

Fixed via the v9 projects API:

```bash
curl -X PATCH "https://api.vercel.com/v9/projects/$PROJECT_ID?teamId=$TEAM_ID" \
  -H "Authorization: Bearer $(jq -r .token ~/.../com.vercel.cli/auth.json)" \
  -H "Content-Type: application/json" \
  -d '{"rootDirectory":"web"}'
```

Next deploy went green in 37s. Two lessons:

1. **CLI `vercel link` only configures the local CLI**, not the
   git integration's build settings. For a Next.js app in a
   subdirectory of a repo, `rootDirectory` must be set
   server-side too.
2. **Watch the Vercel deployments tab, not just halberd-keep**.
   The alias keeps serving the last-good deploy even when 11
   hours of pushes have been failing — there's no visible signal
   on the live site that anything is wrong.

## 2026-05-26 — README demos back to GIF (1280px), mp4 kept as side-by-side download #incident #decision

The mp4 + `<video>` swap was wrong: GitHub's markdown renderer
strips `<video>` tags entirely outside of issue/PR comment context.
The previous commit shipped a README with non-playing video frames
and dropdowns that, when expanded, showed nothing. (Tested
end-to-end on the live README; user caught it within minutes.)

What actually works on GitHub README:

- `![alt](path/to/file.gif)` — GIFs render and autoplay.
- `<video src="...">` — silently stripped by the sanitizer.
- `https://github.com/<owner>/<repo>/raw/...` URLs are fine for
  images but not honored as `<video>` sources.
- Drag-and-drop video into an issue/PR comment yields a
  `user-attachments/assets/...` URL that DOES work inside a
  `<video>` tag — but that's only available for user-attached
  uploads, not repo-committed files.

Course-corrected: kept the mp4 generation in the reporter (for
high-quality downloads), added a GIF generation pass at 1280px
(up from 960px in the original first pass — fixes the user's
resolution complaint) with a 192-color palette and sierra2_4a
dither. README now uses `![…](…)` image syntax for autoplay
inline; each clip's `<sub>` link offers the mp4 alongside for
people who want pixel-perfect quality.

File sizes per clip:

- mp4 (1440×900, libx264 -crf 23 -tune stillimage): ~340–550 KiB
- GIF (1280px, fps 12, palette 192): ~2.2–5.0 MiB

GIFs are well under GitHub's 10 MiB per-asset attach limit, which
means the README ships both formats. Total demo-media footprint
grew but it stays well within reason.

## 2026-05-26 — README demos switched from GIF to H.264 mp4 #decision

First pass shipped the playground demos as GIFs at 960px wide,
~2.2-2.6 MiB each, all gated behind `<details>` dropdowns. Visual
review immediately surfaced two problems: GIFs at 960px made the
playground text fuzzy at 1× display, and putting every demo behind
a click meant first-time visitors saw zero motion before scrolling
past the demo section. Switched both moves:

- **GIF → mp4 (H.264 stillimage tune).** The reporter was already
  producing mp4 as the intermediate before palette-converting to
  GIF; now it stops at mp4 and the GIF step is gone. Crisp 1440×900
  output at ~500 KiB per clip — *smaller* than the lower-res GIFs
  and dramatically sharper because H.264 compresses screen
  recordings with text antialiasing far better than GIF's 256-colour
  palette ever could. Source-of-truth `.cast` for the terminal demo
  re-encoded the same way (187 KiB GIF → 171 KiB mp4).
- **One hero demo inline, alternates in `<details>`.** The Refused
  / DROP TABLE clip is the strongest single artefact, so it
  autoplays muted-and-looped right under the wax-seal triptych. The
  other three (path-traversal refusal, response amendment, safe
  SELECT) stay in `<details>` with `preload="metadata"` and
  `controls` so they don't ship bytes until expanded.

Two implementation footguns worth recording:

- **playwright-bdd drops `=`-bearing Gherkin tags.** Tried
  `@slug=refused-drop-table` for stable filename mapping; the
  generated test files never saw the tags. Switched to a scenario-
  title → slug lookup table in the reporter (uglier but
  bulletproof). Note for future BDD work: tag values via `=` are
  not portable across BDD libraries.
- **GitHub README `<video src>` needs the raw URL, not a relative
  path.** GitHub serves images at relative paths through camo but
  refuses to do the same for video; the workaround is to point
  src at `https://github.com/<owner>/<repo>/raw/main/<path>`.
  Documented as a footnote in the README itself.

## 2026-05-26 — Gherkin demo suite + 4 scenario GIFs in the README #milestone #decision

Halberd now has the full demo-recording pipeline the global CLAUDE.md
baseline calls for: playwright-bdd against the live site, one
`.feature` per journey cluster, a custom reporter that converts
webm → mp4 → palette-aware GIF and skips warmups + 0-byte tests.
Four GIFs landed in `assets/demo/` and are embedded in the README
under collapsed `<details>` clusters:

- **Refused** — DROP TABLE under postgres, path traversal under filesystem
- **Amended** — AWS / GitHub / RSA-laden response under the honeypot
- **Pass granted** — safe SELECT under postgres

Decisions worth recording:

- **Plain Playwright was wrong; Gherkin is right for this repo.** I
  initially started a plain `@playwright/test` setup arguing BDD
  boilerplate wasn't worth it for four scenarios. Course-corrected:
  for a public security repo the `.feature` files *are* the
  threat-coverage documentation — scannable by anyone, not just
  TypeScript readers. The boilerplate buys contributor readability,
  not execution model. Lesson: don't optimize past spec compliance
  without checking the spec's *reason*.
- **`@slug=…` Gherkin tags** map scenarios to stable GIF filenames.
  Without them, the reporter's default slug would be the verbose
  feature-plus-scenario path; with them, `README.md` can reference
  `refused-drop-table.gif` and re-runs land at the same path.
- **Two warmups was the floor; three is safer.** One run still hit
  the 0-byte first-test bug on slot 3 even with two warmups. The
  reporter's defensive `statSync(path).size === 0` check catches the
  spillover and discards instead of shipping a broken GIF. Worth
  remembering: warmup count is best-effort, not a guarantee.
- **`test.afterEach` at module load breaks playwright-bdd's loader.**
  bddgen loads step files outside the test runtime, so a top-level
  `test.afterEach` in `fixtures.ts` throws on import. Moved the
  tail-frame hold to a BDD `After` hook in its own
  `hooks.steps.ts` file.
- **Reporter defers everything to `onEnd`.** `onTestEnd` fires
  before Playwright flushes the video file to disk; only by `onEnd`
  is every webm guaranteed to be written. Same lesson the global
  CLAUDE.md spec spells out, learned in practice.

Total README media inventory now:

- Banner SVG (existing)
- Mermaid sequence diagram (existing)
- Playground hero screenshot (Phase 1)
- Wax-seal triptych (Phase 1)
- Honeypot terminal cast GIF (Phase 1, asciinema → agg)
- 4 Gherkin demo GIFs (Phase 2, this entry)

## 2026-05-26 — Canonical site URL moved to halberd-keep.vercel.app #milestone

Vercel auto-allocated `halberd-six.vercel.app` when the project was
first created (their fallback word-list since plain `halberd.vercel.app`
was claimed elsewhere). Replaced with `halberd-keep.vercel.app`, which
picks up the keep-tour brand we just shipped. The old URL still
resolves; README, `layout.tsx` `metadataBase`, and the GitHub repo
homepage all point at the new one.

Wrinkle worth noting: `vercel alias set <new> <old-alias>` creates an
alias-to-alias CNAME chain, which Vercel's deployment-protection
gates with a 401. The right command is `vercel domains add
<new-alias>` from the linked project directory — that attaches the
alias directly to the latest production deployment and skips the
protection wall. Lesson: for `.vercel.app` aliases, treat them as
project-domain attachments, not deployment redirects.

## 2026-05-26 — Web UI refactor: "the keep tour" medieval reframe #milestone #decision

Halberd's site is now structured as a tour through a fortified keep:

- **I. The Approach** — hero
- **II. The Sentry's Challenge** — playground
- **III. The Threats at the Gate** — threat model
- **IV. The Armory** — bundled rule packs
- **V. The Gatehouse Keys** — install

Left-rail nav (sticky on desktop, hamburger on mobile) tracks the
active section via scroll position. Each section header has a
Roman-numeral marker in Cormorant Garamond display serif, a
ceremonial title, and a smaller functional sans subtitle. Decision
panels in the playground render as **wax seals**: red wax for refused
requests, brass for granted, blue ink for amended responses. Each
rule pack chip carries a small bespoke **heraldic crest** (boar for
postgres, scroll-with-seal for filesystem, branch-graph chevron for
git, octofoil for github, bee for honeypot).

Three calls worth recording:

- **Theme at moments of drama, not everywhere.** Section headers,
  decision verdicts, and rule-pack identity get the medieval
  treatment. Body copy, install commands, code blocks, and threat
  descriptions stay in the existing crisp sans for legibility.
  Picked over "woven motifs" (too restrained) and "auditor's ledger"
  (too localized) options.
- **Refined display serif, not faux-medieval.** Cormorant Garamond
  via next/font/google for ceremonial moments. Cinzel / Uncial /
  blackletter would have tipped into Renaissance Fair cosplay. The
  brass-wax-parchment palette additions live alongside the existing
  cyan/purple tech accents — the contrast IS the brand.
- **Saved the direction to memory.** Future sessions could otherwise
  drift back to the generic cyberpunk-tech default. The memory entry
  at `halberd-keep-tour-brand.md` locks in the section names, palette,
  and the "no faux-medieval fonts / no parchment textures" rules.

Build was straightforward except for one TS type narrowing pinch:
`as const`-tagged section list made `current = sections[0].id` infer
as a single-literal type, blocking later assignments. Widened the
local to `string`. Live deploy on halberd-six.vercel.app within
~3 minutes of `vercel deploy --prod`.

## 2026-05-26 — Deployed playground site at halberd-six.vercel.app #milestone

Halberd now has a live homepage at
[halberd-six.vercel.app](https://halberd-six.vercel.app) — a Next.js 16
app with a hero, an interactive playground, a threat-model summary, and
install instructions. The playground runs the actual `internal/policy`
engine compiled to WebAssembly (via a new `cmd/halberd-wasm` package
that uses `syscall/js`), so decisions in the browser match what the
binaries do byte-for-byte. Five rule packs preloaded with preset
attacks: DROP TABLE, path traversal, --upload-pack ref smuggling,
org-allowlist misses, secret-laden responses.

Decisions worth recording:

- **WASM commits to git, not built on Vercel.** Vercel's build env is
  Node-first; bootstrapping Go in the build command is fragile.
  Committing the 4.6 MiB `web/public/halberd.wasm` (compresses to
  ~1.4 MiB over the wire with Vercel's brotli) is the pragmatic
  trade-off. CI's `web` job rebuilds the WASM on every push and fails
  loudly if the committed artifact drifts from a fresh build — so the
  playground can't quietly ship stale rules.
- **`//go:embed` forbids `..` paths.** The rule packs live at the repo
  root in `policies/`, but `cmd/halberd-wasm/main.go` needs to embed
  them. `scripts/build-wasm.sh` stages a copy into
  `cmd/halberd-wasm/policies/` (gitignored) before building. Cleaner
  than restructuring the repo around the WASM build.
- **pnpm 11's build-script approval blocks scaffolding.** `create-next-app`
  scaffolds with pnpm by default; pnpm 11 then refuses `sharp` and
  `unrs-resolver` build scripts without explicit `onlyBuiltDependencies`
  approval — the old `pnpm` field in `package.json` is deprecated and
  the workspace-yaml location only works inside a workspace root.
  Switched to npm; `package-lock.json` checked in. v0.1 doesn't need
  pnpm's perf wins.
- **Vercel auto-generated alias is `halberd-six.vercel.app`.** Plain
  `halberd.vercel.app` was taken on the platform, so the project gets
  `halberd-six` as its default alias. Three stable aliases land
  automatically: `halberd-six`, `halberd-sankofa-forge`, and
  `halberd-builder106-sankofa-forge`. Documenting the canonical one in
  the README and `metadataBase`; can swap to a custom domain later
  without code changes elsewhere.

Repo metadata updated: homepage URL set to the live site, README banner
gets a `demo - live` badge.

## 2026-05-26 — Filed upstream API feature request; community/community gates API-created discussions #milestone #incident

Halberd's goreleaser pipeline can do everything except set the GitHub
repo's social-preview image — that's still a web-UI-only operation in
2026. Filed a refreshed feature request at
[community/community#197021](https://github.com/orgs/community/discussions/197021)
(Apps, API and Webhooks → Product Feedback → API) referencing the
3.5-year-old precedent at #32166 and framing the 2026 use case
(agentic release workflows, MCP-server tooling layer, security-tooling
release pipelines). Proposed `PUT /repos/{owner}/{repo}/social-preview`
plus a GraphQL `updateRepository(input: { socialPreview: Upload })`
mirror; downstream landing pads in `cli/cli` and
`github/github-mcp-server` follow once the platform endpoint exists.

**Lesson worth recording**: community/community runs a bot that
auto-closes any discussion missing the `source:ui` label. That label is
only applied by the discussion-template form on the web; the GraphQL
`createDiscussion` mutation bypasses templates entirely, so the bot
killed the first attempt (#197020) ~10 minutes after I filed it via
`gh api`. Re-filed via UI as #197021 — same body, picked the dropdowns
that the template requires. Takeaway: for repos that gate intake on
template labels, the GraphQL discussion API is a footgun. Worth
checking `.github/DISCUSSION_TEMPLATE/*.yml` before automating
discussion creation against any community repo.

## 2026-05-26 — Release infrastructure: social card + goreleaser #milestone

Two release-prep items shipped:

- **Social-preview card** at `assets/social-preview.{svg,png}`, 1200×630.
  Adapts the existing banner's color palette (dark slate, purple→cyan
  accent stripe, halberd silhouette) to the wider 1.9:1 ratio used by
  link-share cards. GitHub's REST API doesn't expose social-preview as
  a settable field — needs manual upload via Settings → Social preview.
  Documented in the README's branding-assets section.
- **goreleaser** config at `.goreleaser.yaml` + release workflow at
  `.github/workflows/release.yml`. Tag-driven (`v*`); builds 4 binaries
  × 4 OS/arch targets (linux/darwin × amd64/arm64), bundles each archive
  with LICENSE/README/CONTRIBUTING + every rule pack + the example
  bundle + threat-model and policy-DSL docs, generates a SHA-256
  checksum manifest, and publishes a GitHub Release. Skipped Windows
  for v0.1 — halberd-stdio's PTY-free stdio handoff isn't tested there
  and the audience is Unix-first.

Two decisions worth recording:

- **`const version` → `var version`.** Goreleaser injects the tag value
  via `-ldflags "-X main.version=…"`, which only works on package vars.
  cmd/halberd and cmd/halberd-honeypot both updated. Snapshot dry-run
  confirms the binary now reports the resolved tag (or `<next>-next-<sha>`
  for snapshot builds).
- **No Docker image yet.** Goreleaser can build images, but Halberd is
  a process-mode proxy that mostly runs alongside its host (Claude
  Desktop, Cursor, etc.), not in a container. Docker shippable later
  when there's a real ops story for the remote / k8s deployment.

Validated locally with `goreleaser release --snapshot --clean
--skip=announce,publish`: 16 cross-compiles + 4 archives + checksums
in 57s, every binary present in every archive, version injection
working. Ready for the first real tag push.

## 2026-05-26 — GitGuardian fired on the honeypot's RSA fixture five minutes after push #incident

The honeypot's `list_users` tool returned a fake RSA block as a T5 fixture — `MIIBOgIBAAJBAKj34Gkx...`, a 64-char base64-shaped body that's far too short to be a real key but realistic enough that GitGuardian's source-level scanner fired on it. The alert (incident #33200784) opened on the public repo within five minutes of the push. Fix: swapped the body for a literal `...[FIXTURE_NOT_A_REAL_KEY]` marker — the same shape `internal/policy/sanitize_test.go` already uses without ever tripping a scanner — while keeping the `BEGIN/END` markers so Halberd's runtime `rsa_private_key` scanner still finds and redacts the block in tool output. That's the distinction the comment now spells out: source-level secret scanners and runtime redaction want opposite things from a fixture, and the source scanner wins because it files an incident publicly. Verified post-fix that `list_users` is still redacted end-to-end through `halberd-stdio` + the matched bundle.

## 2026-05-26 — halberd-honeypot ships #milestone

Added `cmd/halberd-honeypot`: a minimal stdio MCP server (~200 LOC) whose
four tools each embody one of the v0.1 threat categories. Pairs with
`halberd-stdio` and a matched `policies/halberd-honeypot.yaml` bundle for
a one-command end-to-end demo:

```bash
halberd-stdio --policy policies/halberd-honeypot.yaml --audit out.jsonl
              -- halberd-honeypot
```

Pipe in a `DROP TABLE`, a `../../etc/passwd`, or a `list_users` call and
watch Halberd block / redact / audit before the agent sees the response.
Smoke test in this session exercised all three behaviors in one pipe
invocation: two requests blocked with synthetic `-32000` errors, one
response with three redactions (AWS / GitHub / RSA), full audit trail.

Three decisions worth recording:

- **Not in `testdata/`, in `cmd/`.** `testdata/` is Go-specific magic
  that gets excluded from `go build ./...`. The honeypot is a real
  binary that's *intended* to be built and run — just never against
  production data. Calling it `cmd/halberd-honeypot` plus a banner that
  prints "VULNERABLE BY DESIGN" on startup communicates intent better
  than burying it in a fixture directory.
- **No build tag.** I considered `//go:build honeypot` to keep it out
  of default builds. Rejected — the honeypot is a positive feature
  (the test fixture that proves the whole stack works), not an
  embarrassment to hide. Documented prominently, no gating.
- **`tools/list_changed` deferred.** T4 (capability creep) needs a
  stateful interaction sequence: server advertises N tools, agent calls
  one, server then pushes a notification adding tool N+1. The honeypot
  doesn't simulate this in v0.1 — its tool list is static. v0.2 adds a
  trigger tool that emits the notification on demand so operators can
  exercise mid-session inventory drift.

11 honeypot-side tests cover the protocol (initialize, tools/list,
notification suppression, unknown-method errors) and each tool's
threat-shaped output. Combined with the existing transport and policy
suites, Halberd's test surface now exceeds 60 cases.

## 2026-05-26 — P5 rule packs + hardening shipped (v0.1 feature-complete) #milestone

Three new rule packs land alongside the existing postgres pack:
`mcp-server-{filesystem,git,github}`. 28 declared tools total across the
four packs, 19 table-driven scenarios in
`internal/policy/packs_test.go` covering both block-and-allow paths.
Each pack is calibrated against a specific threat set documented in the
file header, not a generic "all tools allowed" stance — write-mutating
tools default to denied, array-arg tools are omitted (v0.1 DSL is
scalar-only), and the github pack ships with a `your-org` placeholder
operators must edit before deploying.

Hardening that came along for the ride:

- **`internal/audit/bus_test.go`**: 9 tests covering JSONL framing, time
  stamping, drop-when-full, post-Stop drop semantics, Stop idempotence,
  ctx-deadline honoring, nil-ctx safety, and a conservation property
  (sent == written + dropped) under 16-goroutine × 256-record load.
  Closed the bus's zero-coverage gap.
- **CI actions bumped to Node-24-native majors** ahead of the June 2,
  2026 deadline: `actions/checkout@v5`, `actions/setup-go@v6`,
  `actions/upload-artifact@v5`. The deprecation warnings drop;
  `golangci/golangci-lint-action@v7` stays put for now (v7 was the
  latest as of the migration to golangci-lint v2 last week).

v0.1 covers four of the five threat categories (T1, T2, T4, T5) over
both HTTP and stdio transports. T3 (out-of-scope I/O) is the v0.2
roadmap. Halberd is feature-complete for v0.1.

## 2026-05-26 — Bus.Record race: random select biased the post-Stop drop #incident

The first audit-bus tests exposed a subtle correctness gap: `Bus.Record`
used a single select that watched `<-done`, the buffered send, and a
default branch all as peers. Go picks randomly among ready cases, so a
post-Stop Record sometimes landed in the still-buffered channel rather
than dropping — but the drain goroutine had already exited, so that
event was silently lost (neither written nor counted). Fix: split into a
priority check on `done` first (return early if closed), then a
non-blocking send attempt. Now Record's contract is "after Stop, every
event is counted as dropped" without race. Lesson: when a select has a
done-signal AND a buffered-send case, the done-signal must be checked
first — otherwise Go's random selection silently breaks the priority
invariant operators expect.

## 2026-05-26 — P4 response inspection shipped #milestone #decision

Halberd now sanitizes JSON-RPC responses in both transports: T1 (tool
poisoning via ANSI / zero-width Unicode) and T5 (secret exfiltration via
AWS / GitHub / RSA keys) are covered on the response side. Sanitize-in-
place strategy, not block-with-error — the agent already invoked the
tool, suppressing the response leaves it confused; redacting bad bits
preserves the call's usefulness. Three load-bearing decisions:

- **JSON-tree walk, not raw-byte regex.** ANSI escapes get encoded as
  `` on the wire, not raw ESC. Zero-width chars may be raw UTF-8
  *or* `​`. Scanning raw bytes would miss the JSON-encoded forms.
  The walker unmarshals the envelope, recursively descends `result`,
  sanitizes each string leaf, and re-marshals. `id`, `jsonrpc`, and
  `error` are kept as `json.RawMessage` so protocol metadata round-trips
  byte-exact.
- **SSE skipped in v0.1.** Buffering a `text/event-stream` body to scan
  it would break the streaming contract; per-event inspection lands in
  P4.5 / v0.2. The HTTP `ModifyResponse` short-circuits on
  `Content-Type: text/event-stream`.
- **Opt-in per bundle.** `response_filters: nil` is the fast path —
  transports skip the response-buffering entirely via
  `engine.HasResponseFilters()`. Bundles that only do request-side
  enforcement pay zero response overhead.

Detection records carry `{kind, path}` only — never the matched secret
itself. Logging the very thing we were redacting would defeat the point.

## 2026-05-26 — JSON fixture gotcha: raw ESC bytes are invalid JSON #incident

First run of the response-inspection tests failed with "expected
modification, got none." Root cause: I had embedded raw `\x1b` bytes in
test JSON fixtures using backtick raw strings. RFC 8259 requires control
characters (U+0000–U+001F) to be escaped in JSON strings; Go's
`json.Unmarshal` rejects unescaped ESC, so `EvaluateResponse` fell
through to its "non-JSON, pass through" branch. Fixed by writing ``
in the JSON — which is also what real MCP servers send on the wire.
Lesson: when authoring JSON-RPC test fixtures, paste the on-the-wire
representation, not the post-decode form.

## 2026-05-26 — P3 stdio transport shipped #milestone #decision

Halberd now wraps stdio MCP servers (Claude Desktop, Cursor, Windsurf).
The `halberd-stdio` binary forks the real server, owns its stdin/stdout/
stderr pipes, and runs the policy engine between the host and child in
both directions. Three correctness decisions worth recording:

- **Plain pipes, not a PTY.** MCP stdio is line-delimited JSON-RPC with
  no terminal semantics, and a PTY's line-discipline translation would
  silently corrupt binary content in tool arguments. Earlier planning
  notes called this a "PTY wrapper" — the implementation is `exec.Cmd`
  pipes with newline framing.
- **Blocked notifications drop silently.** JSON-RPC notifications have
  no `id` and the spec forbids a response, so a blocked notification is
  audited but produces no synthetic error. Blocked requests (with `id`)
  still get a `-32000` JSON-RPC error response with the original id
  preserved.
- **Audit log requires a `--path` flag.** Defaulting to stderr would
  collide with the child server's stderr (which we transparently forward
  to the host), corrupting the audit stream. Operator-aware path is the
  safe default.

## 2026-05-26 — Audit bus send-on-closed-channel race fixed #incident

The new stdio tests caught a real bug in `internal/audit`: `Bus.Stop`
called `close(b.ch)` while `Bus.Record` was still selecting on
`b.ch <- e`. The race detector flagged it; in production this would
panic intermittently when a transport's `Stop` raced with an in-flight
audit. Fixed by switching to a `done` channel — `Stop` closes `done`,
`Record` selects on `done` first and counts a dropped event if the bus
has stopped, and the channel itself is never closed. Property tested
under `-race`: 0 panics across the full suite. Lesson: never close a
channel that has multiple senders without a happens-before guarantee on
all of them.

## 2026-05-26 — Full green CI after four iterations #milestone

After the initial red run, four follow-up commits to get all four jobs
(test, bench, govulncheck, golangci-lint) green: (1) bumped Go from 1.22
to stable for stdlib CVE fixes, (2) discovered golangci-lint 2.12.2 was
built with Go 1.25 and can't parse 1.26's stdlib export data — pinned CI
to 1.25 (still in N-1 patch support), (3) bumped golangci-lint-action v6
→ v7 (v6 doesn't support golangci-lint v2 — CI told me directly), (4)
migrated `.golangci.yml` to v2 schema (formatters split out, `version: "2"`
discriminator), fixed gofmt struct-tag alignment, tightened audit-log
file mode to 0o600 (gosec G302), and added doc comments on every
exported identifier in `internal/audit`, `internal/jsonrpc`,
`internal/policy`, and `internal/transport/http`. Lesson: golangci-lint
loses races with the Go release cycle reliably — always pin to N-1
unless you've verified support for current. Final green run: 26460291188.

## 2026-05-26 — First CI run red on stdlib CVEs and lint nits #incident #decision

First push of the scaffold tripped govulncheck because CI pinned Go 1.22,
which has unpatched CVEs in `crypto/tls`, `crypto/x509`, `net`, and
`net/http` reachable from `http.Server.ListenAndServe` and
`httputil.ReverseProxy.ServeHTTP`. Local Go 1.26.3 doesn't have those.
Decision: keep `go.mod` at `go 1.22` as the floor for downstream users,
but switch CI's `setup-go` to `go-version: stable` so the vulnerability
scan reflects what a fresh install gets. golangci-lint also flagged four
nits in `internal/transport/http/proxy_test.go` (two unused `r
*http.Request` params, two unchecked `w.Write` returns) — fixed in the
same commit `f5c0ff4`.

## 2026-05-26 — First green build + baseline bench numbers #milestone

`go test ./...` and `go test -race ./...` pass on first run after install
(Go 1.26.3 from Homebrew). Build produces both `halberd` and `halberd-http`
binaries cleanly. Initial bench on Apple M1: blocked DROP TABLE evaluates
in **2.6 µs/op at 31 allocs/op**, an allowed SELECT in **4.0 µs/op at 25
allocs/op**. Both an order of magnitude under the 200 µs / 50-alloc
ceilings declared in CONTRIBUTING. The 25–31 allocs/op number is mostly
`json.Unmarshal` of the JSON-RPC params (decoded twice — once in `peek` for
the audit-log tool name, once in `evaluateToolCall`). Single-pass decode is
the obvious future optimization but not load-bearing for v0.1.

## 2026-05-26 — Project kickoff #milestone #decision

Started Halberd as the next cybersecurity project after ClearHash. Goal: a
zero-trust JSON-RPC firewall sitting between an LLM agent and its MCP servers,
inspecting `tools/call` traffic for argument injection, capability creep, and
response-side prompt-injection payloads. Picked Go over OCaml (already
represented by `ocaml_limit`) and over Python (red-teaming harness was the
alternative; deferred). MIT license. Module path
`github.com/Builder106/halberd`.

## 2026-05-26 — Scaffold scope: P1 + P2 only #decision

Considered scaffolding all five phases as empty milestones. Rejected — the
"no half-finished implementations" rule from the global CLAUDE.md applies.
P1 (HTTP reverse proxy + audit bus) and P2 (YAML policy engine with
deny-pattern matching and unknown-tool blocking) ship as real working code in
the first pass; P3 (stdio transport), P4 (response inspection), and P5
(rule packs + hardening) live in the README roadmap.

## 2026-05-26 — Policy DSL: hand-rolled, not JSON Schema #decision

Considered pulling in `xeipuuv/gojsonschema` to validate `tools/call`
arguments against full JSON Schema. Rejected for v0.1: the policy DSL is
intentionally narrow (`type`, `max_length`, `deny_patterns`, `allow_values`)
and a hand-rolled validator keeps the dependency surface to one library
(`yaml.v3`). Reconsider in v0.2 if rule packs need `oneOf` / `anyOf` /
recursive shapes.

## 2026-05-26 — JSON-RPC error code -32000 for policy violations #decision

Picked the `-32000` server-defined error code for policy violations. The
JSON-RPC 2.0 spec reserves `-32000` through `-32099` for "implementation-
defined server errors." Halberd surfaces violations through this code so the
agent's MCP client treats them as recoverable upstream errors and (in
practice) reasons about why the tool failed rather than crashing the session.

## 2026-05-26 — Go toolchain not installed locally #incident

`go` not on PATH at scaffold time. Wrote `go.mod` by hand against Go 1.22.
Next step: `brew install go && cd Halberd && go mod tidy && go test ./...`
to verify the codebase compiles. CI will compile against a fresh toolchain
either way.
