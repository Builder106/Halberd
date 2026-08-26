import { describe, it, expect, vi, beforeEach } from "vitest";
import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { Playground } from "./Playground";
import * as halberdLib from "../lib/halberd";

describe("Playground interactive component", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders error state when loadHalberd fails", async () => {
    vi.spyOn(halberdLib, "loadHalberd").mockRejectedValue(new Error("WASM init failed"));

    render(React.createElement(Playground));

    await waitFor(() => {
      expect(screen.getByText(/The sentry could not be reached: WASM init failed/)).toBeTruthy();
    });
  });

  it("renders ready state and handles request evaluation, pack switching, presets, and error handling", async () => {
    const mockPacks = [
      { name: "mcp-server-postgres", server: "stdio", responseFilters: true },
      { name: "mcp-server-filesystem", server: "stdio", responseFilters: false },
    ];

    const mockHalberdGlobal = {
      packs: () => JSON.stringify(mockPacks),
      evaluateRequest: vi.fn((pack: string, payload: string) => {
        if (payload.includes("DROP TABLE")) {
          return JSON.stringify({
            Blocked: true,
            Violations: [
              {
                Category: "sql",
                Tool: "query",
                Field: "sql",
                Rule: "drop",
                Detail: "DROP TABLE forbidden",
              },
            ],
          });
        }
        return JSON.stringify({ Blocked: false, Violations: null });
      }),
      evaluateResponse: vi.fn((pack: string, payload: string) => {
        if (payload.includes("aws_key")) {
          return JSON.stringify({
            modified: true,
            payload: '{"result":{"content":[{"type":"text","text":"[REDACTED]"}]}}',
            detections: [{ kind: "aws_access_key", path: "result.content[0].text" }],
          });
        }
        return JSON.stringify({ modified: false, payload, detections: null });
      }),
      version: "0.1.0-wasm",
    };

    vi.spyOn(halberdLib, "loadHalberd").mockResolvedValue(mockHalberdGlobal as never);
    (globalThis as typeof globalThis & { halberd?: typeof mockHalberdGlobal }).halberd = mockHalberdGlobal;

    render(React.createElement(Playground));

    await waitFor(() => {
      expect(screen.getByText(/Garrison · which sentry stands the watch/)).toBeTruthy();
    });

    // Test evaluating an allowed request
    const challengeBtn = screen.getByRole("button", { name: /Challenge the envelope/i });
    fireEvent.click(challengeBtn);

    await waitFor(() => {
      expect(screen.getByText("Pass granted")).toBeTruthy();
    });

    // Test applying a preset that gets blocked
    const dropPresetBtn = screen.getByRole("button", { name: /DROP TABLE/i });
    fireEvent.click(dropPresetBtn);
    fireEvent.click(challengeBtn);

    await waitFor(() => {
      expect(screen.getByText("Refused")).toBeTruthy();
      expect(screen.getByText("DROP TABLE forbidden")).toBeTruthy();
    });

    // Test switching to response direction
    const responseTab = screen.getByRole("button", { name: /response — outbound/i });
    fireEvent.click(responseTab);

    const secretPresetBtn = screen.getByRole("button", { name: /Response with embedded AWS key/i });
    fireEvent.click(secretPresetBtn);
    fireEvent.click(challengeBtn);

    await waitFor(() => {
      expect(screen.getByText("Amended")).toBeTruthy();
      expect(screen.getByText("aws_access_key")).toBeTruthy();
    });

    // Test switching pack
    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "mcp-server-filesystem" } });

    // Test exception handling in evaluate
    mockHalberdGlobal.evaluateRequest.mockImplementationOnce(() => {
      throw new Error("Synthetic evaluation error");
    });
    const requestTab = screen.getByRole("button", { name: /request — inbound/i });
    fireEvent.click(requestTab);
    fireEvent.click(challengeBtn);

    await waitFor(() => {
      expect(screen.getByText("Synthetic evaluation error")).toBeTruthy();
    });
  });
});
