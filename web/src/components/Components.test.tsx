import { describe, it, expect } from "vitest";
import React from "react";
import { renderToString } from "react-dom/server";
import { HalberdMark } from "./HalberdMark";
import { SectionMarker } from "./SectionMarker";
import { Footer } from "./Footer";
import { Install } from "./Install";
import { KeepNav } from "./KeepNav";

describe("Auxiliary components", () => {
  it("renders HalberdMark", () => {
    const html = renderToString(React.createElement(HalberdMark, { size: 100 }));
    expect(html).toContain('aria-label="Halberd mark"');
  });

  it("renders SectionMarker", () => {
    const html = renderToString(
      React.createElement(SectionMarker, {
        numeral: "I",
        ceremonial: "The Test Keep",
        functional: "Testing section marker",
      })
    );
    expect(html).toContain("The Test Keep");
    expect(html).toContain("Testing section marker");
    expect(html).toContain("I");
  });

  it("renders Footer", () => {
    const html = renderToString(React.createElement(Footer));
    expect(html).toContain("Halberd — MIT licensed.");
    expect(html).toContain("threat model");
    expect(html).toContain("policy DSL");
  });

  it("renders Install", () => {
    const html = renderToString(React.createElement(Install));
    expect(html).toContain("The Gatehouse Keys");
    expect(html).toContain("Download a release");
    expect(html).toContain("Build from source");
  });

  it("renders KeepNav", () => {
    const html = renderToString(React.createElement(KeepNav));
    expect(html).toContain("THE KEEP");
    expect(html).toContain("The Approach");
    expect(html).toContain("The Sentry&#x27;s Challenge");
  });
});
