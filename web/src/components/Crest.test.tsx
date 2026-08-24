import { describe, it, expect } from "vitest";
import React from "react";
import { renderToString } from "react-dom/server";
import { Crest } from "./Crest";

describe("Crest component", () => {
  it("renders crest for postgres", () => {
    const html = renderToString(React.createElement(Crest, { pack: "mcp-server-postgres" }));
    expect(html).toContain("<svg");
  });

  it("renders crest for filesystem", () => {
    const html = renderToString(React.createElement(Crest, { pack: "mcp-server-filesystem" }));
    expect(html).toContain("<svg");
  });

  it("renders crest for git", () => {
    const html = renderToString(React.createElement(Crest, { pack: "mcp-server-git" }));
    expect(html).toContain("<svg");
  });

  it("renders crest for github", () => {
    const html = renderToString(React.createElement(Crest, { pack: "mcp-server-github" }));
    expect(html).toContain("<svg");
  });

  it("renders crest for honeypot", () => {
    const html = renderToString(React.createElement(Crest, { pack: "halberd-honeypot" }));
    expect(html).toContain("<svg");
  });

  it("renders fallback crest for unknown pack", () => {
    const html = renderToString(React.createElement(Crest, { pack: "custom-unknown" }));
    expect(html).toContain("<svg");
  });
});
