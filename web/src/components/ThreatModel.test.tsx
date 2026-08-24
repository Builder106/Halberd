import { describe, it, expect } from "vitest";
import React from "react";
import { renderToString } from "react-dom/server";
import { Threats, Armory } from "./ThreatModel";

describe("ThreatModel components", () => {
  it("renders Threats component with all threat categories", () => {
    const html = renderToString(React.createElement(Threats));
    expect(html).toContain("The Threats at the Gate");
    expect(html).toContain("Tool poisoning");
    expect(html).toContain("Argument injection");
    expect(html).toContain("Out-of-scope I/O");
    expect(html).toContain("Capability creep");
    expect(html).toContain("Exfiltration via response");
  });

  it("renders Armory component with pre-forged bundles", () => {
    const html = renderToString(React.createElement(Armory));
    expect(html).toContain("The Armory");
    expect(html).toContain("mcp-server-postgres");
    expect(html).toContain("mcp-server-filesystem");
    expect(html).toContain("mcp-server-git");
    expect(html).toContain("mcp-server-github");
    expect(html).toContain("halberd-honeypot");
  });
});
