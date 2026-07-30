import { HalberdMark } from "./HalberdMark";

// I. The Approach — first encounter with the keep. Halberd silhouette
// stays the visual anchor; the wordmark and tagline are unchanged,
// but a single ceremonial line ("Every request must pass the gate.")
// sits above the existing technical pipeline line.
export function Hero() {
  return (
    <section
      id="approach"
      className="relative overflow-hidden border-b border-(--color-border)"
    >
      <div className="absolute inset-0 grid-bg opacity-60" />
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          background:
            "radial-gradient(60% 60% at 18% 50%, rgba(124,77,255,0.18), transparent 70%), radial-gradient(45% 45% at 80% 25%, rgba(176,137,72,0.10), transparent 70%)",
        }}
      />

      <div className="relative max-w-5xl mx-auto px-6 pt-24 pb-32 flex flex-col md:flex-row items-center gap-16">
        <div className="relative shrink-0">
          {/* Faint gate silhouette behind the halberd — drawn as two
              crenellated towers framing the polearm. Reads as
              architecture, not fantasy. */}
          <svg
            viewBox="0 0 240 520"
            width={240}
            height={520}
            aria-hidden
            className="absolute inset-0 opacity-25"
          >
            <g fill="var(--color-grid)">
              {/* left tower */}
              <rect x="10" y="80" width="50" height="440" />
              <rect x="10" y="70" width="10" height="20" />
              <rect x="30" y="70" width="10" height="20" />
              <rect x="50" y="70" width="10" height="20" />
              {/* right tower */}
              <rect x="180" y="80" width="50" height="440" />
              <rect x="180" y="70" width="10" height="20" />
              <rect x="200" y="70" width="10" height="20" />
              <rect x="220" y="70" width="10" height="20" />
              {/* gate arch */}
              <path d="M60 200 Q120 140 180 200 L180 520 L60 520 Z" opacity="0.5" />
            </g>
          </svg>
          <HalberdMark size={260} className="relative z-10" />
        </div>

        <div className="flex-1 max-w-3xl">
          <div className="inline-flex items-center gap-2 px-3 py-1 mb-6 text-xs font-mono rounded-full border border-(--color-brass)/40 bg-(--color-panel) text-(--color-brass)">
            <span className="w-1.5 h-1.5 rounded-full bg-(--color-brass)" />
            GARRISON · v0.1 · MIT · PRE-RELEASE
          </div>

          <h1
            className="text-6xl md:text-7xl font-extrabold tracking-tight leading-none mb-5"
            style={{ fontFamily: "var(--font-display)" }}
          >
            HALBERD
          </h1>

          <div
            className="w-32 h-1 mb-6 rounded-full"
            style={{
              background:
                "linear-gradient(90deg, var(--color-accent), var(--color-accent-2))",
            }}
          />

          <p
            className="text-2xl md:text-3xl text-(--color-fg) mb-3 italic"
            style={{ fontFamily: "var(--font-serif)" }}
          >
            Every request must pass the gate.
          </p>
          <p className="text-lg text-(--color-fg-2) mb-3">
            A JSON-RPC firewall for MCP agents.
          </p>
          <p className="text-base font-mono text-(--color-fg-3) mb-10">
            tools/call → policy → audit → upstream
          </p>

          <div className="flex flex-wrap gap-3">
            <a
              href="#sentry"
              className="inline-flex items-center px-5 py-2.5 rounded-md font-medium text-(--color-bg) bg-(--color-fg) hover:opacity-90 transition"
            >
              Approach the sentry →
            </a>
            <a
              href="https://github.com/Builder106/halberd"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center px-5 py-2.5 rounded-md font-medium text-(--color-fg) border border-(--color-border) hover:bg-(--color-panel) transition"
            >
              View on GitHub ↗
            </a>
            <a
              href="#gatehouse"
              className="inline-flex items-center px-5 py-2.5 rounded-md font-medium text-(--color-fg-2) hover:text-(--color-fg) transition"
            >
              Take the keys
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}
