import { describe, it, expect } from "vitest";
import React from "react";
import { renderToString } from "react-dom/server";
import { Hero } from "./Hero";

describe("Hero component", () => {
  it("renders main heading, subtitle, and CTA links", () => {
    const html = renderToString(React.createElement(Hero));
    expect(html).toContain("HALBERD");
    expect(html).toContain("Every request must pass the gate.");
    expect(html).toContain("A JSON-RPC firewall for MCP agents.");
    expect(html).toContain("tools/call → policy → audit → upstream");
    expect(html).toContain("Approach the sentry →");
    expect(html).toContain("View on GitHub ↗");
    expect(html).toContain("Take the keys");
  });
});
