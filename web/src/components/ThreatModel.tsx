import { SectionMarker } from "./SectionMarker";
import { Crest } from "./Crest";

const threats = [
  {
    id: "T1",
    chronicle: "I",
    name: "Tool poisoning",
    coverage: "covered, v0.1",
    summary:
      "A compromised MCP server slips role-tag spoofs, ANSI escapes, or zero-width Unicode into a tool response to hijack the agent's next turn.",
    halberd: "Response inspector strikes ANSI and zero-width Unicode in place.",
  },
  {
    id: "T2",
    chronicle: "II",
    name: "Argument injection",
    coverage: "covered, v0.1",
    summary:
      "The agent is induced to call a legitimate tool with hostile arguments: DROP TABLE, --upload-pack=…, statement chaining via ;--.",
    halberd:
      "Per-tool regex denylist, type checks, max-length, and allowlist enums.",
  },
  {
    id: "T3",
    chronicle: "III",
    name: "Out-of-scope I/O",
    coverage: "v0.2 roadmap",
    summary:
      "A narrow-purpose tool gets pushed to read /etc/shadow, hit a private IP, or write outside its sandbox.",
    halberd:
      "Path-traversal, absolute-path, and home-expansion patterns already deny on filesystem and git packs.",
  },
  {
    id: "T4",
    chronicle: "IV",
    name: "Capability creep",
    coverage: "covered, v0.1",
    summary:
      'Mid-session, an MCP server pushes "tools/list_changed" and adds a tool the agent calls before any human reviews it.',
    halberd:
      "Tool inventory pinned to the bundle; any unknown tool is denied by default.",
  },
  {
    id: "T5",
    chronicle: "V",
    name: "Exfiltration via response",
    coverage: "covered, v0.1",
    summary:
      "A tool response carries AWS keys, GitHub tokens, or RSA private keys back into the model context.",
    halberd:
      "Built-in scanners redact aws_access_key, github_token, and rsa_private_key on the wire.",
  },
];

export function Threats() {
  return (
    <section
      id="threats"
      className="relative max-w-5xl mx-auto px-6 py-24 border-b border-(--color-border)"
    >
      <SectionMarker
        numeral="III"
        ceremonial="The Threats at the Gate"
        functional="The five named threat categories"
      />
      <p className="text-(--color-fg-2) mb-12 max-w-2xl">
        Halberd v0.1 ships request- and response-side coverage for four of
        the five threats below, on both the HTTP and stdio transports.
      </p>

      <div className="grid md:grid-cols-2 gap-4">
        {threats.map((t) => (
          <div
            key={t.id}
            className="p-5 rounded-lg border border-(--color-border) bg-(--color-panel)/40 relative"
          >
            <span
              aria-hidden
              className="absolute right-4 top-3 font-serif text-2xl text-(--color-fg-3)/50 select-none"
            >
              {t.chronicle}
            </span>
            <div className="flex items-center justify-between mb-3 pr-8">
              <div className="flex items-center gap-3">
                <span className="font-mono text-(--color-fg-3) text-sm">
                  {t.id}
                </span>
                <h3
                  className="font-semibold text-lg"
                  style={{ fontFamily: "var(--font-serif)" }}
                >
                  {t.name}
                </h3>
              </div>
              <span
                className={`text-xs font-mono px-2 py-0.5 rounded ${
                  t.coverage.startsWith("covered")
                    ? "text-(--color-brass) bg-(--color-brass)/10 border border-(--color-brass)/30"
                    : "text-(--color-warning) bg-(--color-warning)/10 border border-(--color-warning)/30"
                }`}
              >
                {t.coverage}
              </span>
            </div>
            <p className="text-sm text-(--color-fg-2) mb-3">{t.summary}</p>
            <p className="text-sm text-(--color-fg-3)">
              <span className="text-(--color-fg-2) font-medium">
                The garrison:{" "}
              </span>
              {t.halberd}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

const packs = [
  { name: "mcp-server-postgres", tools: 3 },
  { name: "mcp-server-filesystem", tools: 9 },
  { name: "mcp-server-git", tools: 8 },
  { name: "mcp-server-github", tools: 8 },
  {
    name: "halberd-honeypot",
    tools: 4,
    hint: "deliberately-vulnerable demo target",
  },
];

export function Armory() {
  return (
    <section
      id="armory"
      className="relative max-w-5xl mx-auto px-6 py-24 border-b border-(--color-border)"
    >
      <SectionMarker
        numeral="IV"
        ceremonial="The Armory"
        functional="Bundled rule packs"
      />
      <p
        className="text-(--color-fg) mb-3 italic text-lg"
        style={{ fontFamily: "var(--font-serif)" }}
      >
        Pre-forged bundles. Carry one to the gate, or forge your own.
      </p>
      <p className="text-(--color-fg-2) mb-10 max-w-2xl">
        Each pack is a YAML policy bundle calibrated against a specific
        MCP server. The DSL is intentionally narrow:{" "}
        <code>type</code> · <code>max_length</code> · <code>allow_values</code>{" "}
        · <code>deny_patterns</code> · response-side secret scanners.
      </p>

      <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {packs.map((p) => (
          <a
            key={p.name}
            href={`https://github.com/Builder106/halberd/blob/main/policies/${p.name}.yaml`}
            target="_blank"
            rel="noreferrer"
            className="group flex items-start gap-3 p-4 rounded-md border border-(--color-border) bg-(--color-panel)/40 hover:bg-(--color-panel) hover:border-(--color-brass)/40 transition"
          >
            <Crest pack={p.name} size={24} className="shrink-0 mt-0.5" />
            <div className="min-w-0 flex-1">
              <div className="font-mono text-sm text-(--color-fg) truncate">
                {p.name}
              </div>
              <div className="text-xs text-(--color-fg-3) mt-0.5">
                {p.tools} tools{p.hint ? " · " + p.hint : ""}
              </div>
            </div>
            <span
              aria-hidden
              className="font-mono text-xs text-(--color-fg-3) opacity-0 group-hover:opacity-100 transition mt-0.5"
            >
              ↗
            </span>
          </a>
        ))}
      </div>
    </section>
  );
}
