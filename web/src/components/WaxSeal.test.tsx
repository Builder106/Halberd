import { describe, it, expect } from "vitest";
import React from "react";
import { renderToString } from "react-dom/server";
import { WaxSeal } from "./WaxSeal";

describe("WaxSeal component", () => {
  it("renders refused variant correctly", () => {
    const html = renderToString(React.createElement(WaxSeal, { variant: "refused", size: 100 }));
    expect(html).toContain("REFUSED · BY ORDER OF THE POLICY");
    expect(html).toContain('aria-label="Halberd verdict: refused"');
    expect(html).toContain('id="wax-refused"');
  });

  it("renders granted variant correctly", () => {
    const html = renderToString(React.createElement(WaxSeal, { variant: "granted" }));
    expect(html).toContain("PASS GRANTED · ADMITTANCE");
    expect(html).toContain('aria-label="Halberd verdict: granted"');
    expect(html).toContain('id="wax-granted"');
  });

  it("renders amended variant correctly", () => {
    const html = renderToString(React.createElement(WaxSeal, { variant: "amended" }));
    expect(html).toContain("AMENDED · BY THE AUDITOR&#x27;S HAND");
    expect(html).toContain('aria-label="Halberd verdict: amended"');
    expect(html).toContain('id="wax-amended"');
  });

  it("hides inscription when showInscription is false", () => {
    const html = renderToString(
      React.createElement(WaxSeal, { variant: "refused", showInscription: false })
    );
    expect(html).not.toContain("REFUSED · BY ORDER OF THE POLICY");
  });
});
