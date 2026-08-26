import { describe, it, expect } from "vitest";
import { presets } from "./presets";

describe("presets", () => {
  it("contains required rule packs", () => {
    const requiredPacks = [
      "mcp-server-postgres",
      "mcp-server-filesystem",
      "mcp-server-git",
      "mcp-server-github",
      "halberd-honeypot",
    ];

    for (const pack of requiredPacks) {
      expect(presets[pack]).toBeDefined();
      expect(presets[pack].length).toBeGreaterThan(0);
    }
  });

  it("ensures every preset has valid JSON payload and valid properties", () => {
    for (const [, presetList] of Object.entries(presets)) {
      for (const preset of presetList) {
        expect(preset.id).toBeTruthy();
        expect(preset.label).toBeTruthy();
        expect(["request", "response"]).toContain(preset.direction);
        expect(["block", "allow", "sanitize"]).toContain(preset.expect);

        // Verify JSON payload parses cleanly
        expect(() => JSON.parse(preset.payload)).not.toThrow();
        const parsed = JSON.parse(preset.payload);
        expect(parsed.jsonrpc).toBe("2.0");
      }
    }
  });
});
