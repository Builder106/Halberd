"use client";
import { useState } from "react";
import { SectionMarker } from "./SectionMarker";

function CopyableBlock({ children }: { children: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="relative group">
      <pre className="font-mono text-sm bg-(--color-panel) border border-(--color-border) rounded-lg p-4 overflow-x-auto whitespace-pre">
        {children}
      </pre>
      <button
        onClick={() => {
          navigator.clipboard.writeText(children);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        }}
        className="absolute top-2 right-2 px-2 py-1 text-xs font-mono rounded border border-(--color-border) bg-(--color-bg-2) text-(--color-fg-2) opacity-0 group-hover:opacity-100 hover:text-(--color-fg) transition"
        aria-label="Copy"
      >
        {copied ? "copied" : "copy"}
      </button>
    </div>
  );
}

export function Install() {
  return (
    <section
      id="gatehouse"
      className="relative max-w-5xl mx-auto px-6 py-24 border-b border-(--color-border)"
    >
      <SectionMarker
        numeral="V"
        ceremonial="The Gatehouse Keys"
        functional="Install Halberd at your own gate"
      />
      <p
        className="text-(--color-fg) mb-3 italic text-lg"
        style={{ fontFamily: "var(--font-serif)" }}
      >
        Take these and stand a watch of your own.
      </p>
      <p className="text-(--color-fg-2) mb-12 max-w-2xl">
        Pre-built binaries ship for linux and darwin × amd64 and arm64. Each
        archive bundles all four binaries plus every rule pack.
      </p>

      <div className="grid md:grid-cols-2 gap-8">
        <div>
          <h3
            className="font-semibold mb-3 text-(--color-fg)"
            style={{ fontFamily: "var(--font-serif)" }}
          >
            <span className="text-(--color-brass) mr-2" aria-hidden>
              ❦
            </span>
            Download a release
          </h3>
          <CopyableBlock>{`curl -L https://github.com/Builder106/halberd/releases/latest/download/\\
  halberd_\${VERSION}_\${OS}_\${ARCH}.tar.gz | tar -xz
./halberd version`}</CopyableBlock>
        </div>

        <div>
          <h3
            className="font-semibold mb-3 text-(--color-fg)"
            style={{ fontFamily: "var(--font-serif)" }}
          >
            <span className="text-(--color-brass) mr-2" aria-hidden>
              ❦
            </span>
            Build from source
          </h3>
          <CopyableBlock>{`brew install go
git clone https://github.com/Builder106/halberd && cd Halberd
go build -o bin/ ./cmd/...
./bin/halberd lint policies/mcp-server-postgres.yaml`}</CopyableBlock>
        </div>

        <div>
          <h3
            className="font-semibold mb-3 text-(--color-fg)"
            style={{ fontFamily: "var(--font-serif)" }}
          >
            <span className="text-(--color-brass) mr-2" aria-hidden>
              ❦
            </span>
            Wrap a local stdio server (Claude Desktop)
          </h3>
          <p className="text-sm text-(--color-fg-3) mb-3">
            Edit{" "}
            <code>~/Library/Application Support/Claude/claude_desktop_config.json</code>{" "}
            — point at <code>halberd-stdio</code> instead of the real server:
          </p>
          <CopyableBlock>{`"mcpServers": {
  "postgres": {
    "command": "/usr/local/bin/halberd-stdio",
    "args": [
      "--policy", "/etc/halberd/postgres.yaml",
      "--audit",  "/var/log/halberd/postgres.jsonl",
      "--", "mcp-server-postgres", "--conn-string", "postgresql://..."
    ]
  }
}`}</CopyableBlock>
        </div>

        <div>
          <h3
            className="font-semibold mb-3 text-(--color-fg)"
            style={{ fontFamily: "var(--font-serif)" }}
          >
            <span className="text-(--color-brass) mr-2" aria-hidden>
              ❦
            </span>
            HTTP transport (remote MCP)
          </h3>
          <p className="text-sm text-(--color-fg-3) mb-3">
            Sit between your agent and a remote MCP server:
          </p>
          <CopyableBlock>{`halberd-http \\
  --policy policies/mcp-server-postgres.yaml \\
  --target http://upstream:8080 \\
  --listen :9090 \\
  --audit  halberd.jsonl`}</CopyableBlock>
        </div>
      </div>
    </section>
  );
}
