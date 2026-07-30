"use client";

import { useEffect, useState } from "react";

const sections = [
  { id: "approach", numeral: "I", title: "The Approach" },
  { id: "sentry", numeral: "II", title: "The Sentry's Challenge" },
  { id: "threats", numeral: "III", title: "The Threats at the Gate" },
  { id: "armory", numeral: "IV", title: "The Armory" },
  { id: "gatehouse", numeral: "V", title: "The Gatehouse Keys" },
] as const;

export function KeepNav() {
  const [active, setActive] = useState<string>("approach");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    // Pick the section whose top is closest to (but not below) the
    // viewport's top quarter. Reads from scroll position rather than
    // IntersectionObserver because we want a single canonical "active"
    // even when multiple sections are visible at once.
    function onScroll() {
      const probe = window.scrollY + window.innerHeight * 0.25;
      let current: string = sections[0].id;
      for (const s of sections) {
        const el = document.getElementById(s.id);
        if (el && el.offsetTop <= probe) current = s.id;
      }
      setActive(current);
    }
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <>
      {/* Mobile: top bar with hamburger that reveals an in-page sheet. */}
      <div className="md:hidden sticky top-0 z-30 flex items-center justify-between px-4 py-3 bg-(--color-bg)/95 backdrop-blur border-b border-(--color-border)">
        <button
          onClick={() => setOpen((o) => !o)}
          className="font-mono text-xs text-(--color-fg-2) flex items-center gap-2"
          aria-label="Toggle keep navigation"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden>
            <rect x="1" y="3" width="14" height="1.5" fill="currentColor" />
            <rect x="1" y="7.25" width="14" height="1.5" fill="currentColor" />
            <rect x="1" y="11.5" width="14" height="1.5" fill="currentColor" />
          </svg>
          THE KEEP
        </button>
        <span className="font-serif text-(--color-brass) text-sm">
          {sections.find((s) => s.id === active)?.numeral}.{" "}
          <span className="text-(--color-fg-2)">
            {sections.find((s) => s.id === active)?.title}
          </span>
        </span>
      </div>
      {open && (
        <div className="md:hidden fixed inset-x-0 top-[49px] z-20 border-b border-(--color-border) bg-(--color-bg-2)">
          <ul className="px-4 py-3 space-y-1">
            {sections.map((s) => (
              <li key={s.id}>
                <a
                  href={`#${s.id}`}
                  onClick={() => setOpen(false)}
                  className={`flex items-baseline gap-3 py-2 ${
                    active === s.id ? "text-(--color-fg)" : "text-(--color-fg-2)"
                  }`}
                >
                  <span className="font-serif text-(--color-brass) w-8 tabular-nums">
                    {s.numeral}
                  </span>
                  <span>{s.title}</span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Desktop: fixed left rail. */}
      <nav className="hidden md:flex fixed left-0 top-0 bottom-0 w-56 flex-col border-r border-(--color-border) bg-(--color-bg)/85 backdrop-blur-sm z-20">
        <a
          href="#approach"
          className="flex items-center gap-2 px-5 pt-6 pb-8 text-(--color-fg) hover:text-(--color-fg) group"
        >
          <span
            className="font-serif text-(--color-brass) text-xl"
            aria-hidden
          >
            ⚔
          </span>
          <span className="font-bold tracking-wide">HALBERD</span>
          <span className="ml-auto font-mono text-[10px] text-(--color-fg-3) group-hover:text-(--color-fg-2)">
            v0.1
          </span>
        </a>

        <span
          className="px-5 pb-3 text-[10px] font-mono tracking-[0.2em] text-(--color-fg-3)"
          aria-hidden
        >
          THE KEEP
        </span>

        <ul className="flex-1">
          {sections.map((s) => {
            const isActive = active === s.id;
            return (
              <li key={s.id}>
                <a
                  href={`#${s.id}`}
                  className={`group relative flex items-baseline gap-4 px-5 py-2.5 transition ${
                    isActive
                      ? "text-(--color-fg)"
                      : "text-(--color-fg-2) hover:text-(--color-fg)"
                  }`}
                >
                  {isActive && (
                    <span
                      aria-hidden
                      className="absolute left-0 top-2 bottom-2 w-[3px] bg-(--color-brass) rounded-r"
                    />
                  )}
                  <span
                    className={`font-serif w-8 tabular-nums text-lg ${
                      isActive ? "text-(--color-brass)" : "text-(--color-fg-3)"
                    }`}
                  >
                    {s.numeral}
                  </span>
                  <span className="text-sm">{s.title}</span>
                </a>
              </li>
            );
          })}
        </ul>

        <div className="px-5 py-5 border-t border-(--color-border) text-[10px] font-mono text-(--color-fg-3) leading-relaxed">
          A JSON-RPC firewall for MCP agents.
          <br />
          <a
            href="https://github.com/Builder106/halberd"
            target="_blank"
            rel="noreferrer"
            className="text-(--color-fg-2) hover:text-(--color-fg)"
          >
            github.com/Builder106/halberd ↗
          </a>
        </div>
      </nav>
    </>
  );
}
