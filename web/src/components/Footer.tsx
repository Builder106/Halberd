export function Footer() {
  return (
    <footer className="max-w-6xl mx-auto px-6 py-12 text-sm text-(--color-fg-3)">
      <div className="flex flex-col md:flex-row gap-4 justify-between items-start md:items-center">
        <p>
          Halberd — MIT licensed. Source at{" "}
          <a
            href="https://github.com/Builder106/halberd"
            target="_blank"
            rel="noreferrer"
            className="text-(--color-fg-2) hover:text-(--color-fg) underline underline-offset-2"
          >
            github.com/Builder106/halberd
          </a>
          .
        </p>
        <nav className="flex gap-5">
          <a
            href="https://github.com/Builder106/halberd/blob/main/docs/threat-model.md"
            target="_blank"
            rel="noreferrer"
            className="hover:text-(--color-fg-2) transition"
          >
            threat model
          </a>
          <a
            href="https://github.com/Builder106/halberd/blob/main/docs/policy-dsl.md"
            target="_blank"
            rel="noreferrer"
            className="hover:text-(--color-fg-2) transition"
          >
            policy DSL
          </a>
          <a
            href="https://github.com/Builder106/halberd/blob/main/JOURNAL.md"
            target="_blank"
            rel="noreferrer"
            className="hover:text-(--color-fg-2) transition"
          >
            journal
          </a>
          <a
            href="https://github.com/Builder106/halberd/releases"
            target="_blank"
            rel="noreferrer"
            className="hover:text-(--color-fg-2) transition"
          >
            releases
          </a>
        </nav>
      </div>
    </footer>
  );
}
