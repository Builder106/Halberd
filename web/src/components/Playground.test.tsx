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

    // Test typing in textarea directly
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "invalid json string" } });

    // Test evaluating modified response with non-JSON original payload to test tryPretty catch
    mockHalberdGlobal.evaluateResponse.mockReturnValueOnce(
      JSON.stringify({
        modified: true,
        payload: '{"result":{"content":[{"type":"text","text":"[REDACTED]"}]}}',
        detections: [{ kind: "aws_access_key", path: "result.content[0].text" }],
      }),
    );
    fireEvent.click(challengeBtn);
    await waitFor(() => {
      expect(screen.getByText("Amended")).toBeTruthy();
    });

    // Test evaluating an unmodified response
    mockHalberdGlobal.evaluateResponse.mockReturnValueOnce(
      JSON.stringify({
        modified: false,
        payload: '{"status":"ok"}',
        detections: null,
      }),
    );
    fireEvent.click(responseTab);
    fireEvent.click(challengeBtn);
    await waitFor(() => {
      expect(screen.getByText("Passed through")).toBeTruthy();
      expect(screen.getByText(/the response reached the agent unchanged/)).toBeTruthy();
    });

    // Test switching to a pack without presets to cover setPayload("")
    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "mcp-server-filesystem" } });
    mockPacks.push({ name: "mcp-server-custom-nopresets", server: "stdio", responseFilters: false });
    fireEvent.change(select, { target: { value: "mcp-server-custom-nopresets" } });

    // Test exception handling with Error instance
    mockHalberdGlobal.evaluateRequest.mockImplementationOnce(() => {
      throw new Error("Synthetic Error object");
    });
    fireEvent.click(challengeBtn);
    await waitFor(() => {
      expect(screen.getByText("Synthetic Error object")).toBeTruthy();
    });

    // Test exception handling with non-Error thrown
    mockHalberdGlobal.evaluateRequest.mockImplementationOnce(() => {
      throw "Synthetic string error";
    });
    fireEvent.click(challengeBtn);
    await waitFor(() => {
      expect(screen.getByText("Synthetic string error")).toBeTruthy();
    });

    // Test evaluate when halberd global is missing
    const savedHalberd = globalThis.halberd;
    delete (globalThis as { halberd?: unknown }).halberd;
    fireEvent.click(challengeBtn);
    globalThis.halberd = savedHalberd;
  });

  it("ignores resolve when unmounted during load", async () => {
    let resolvePromise!: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    vi.spyOn(halberdLib, "loadHalberd").mockReturnValue(
      promise as unknown as ReturnType<typeof halberdLib.loadHalberd>,
    );

    const { unmount } = render(React.createElement(Playground));
    unmount();
    resolvePromise({
      packs: () => "[]",
      evaluateRequest: vi.fn(),
      evaluateResponse: vi.fn(),
      version: "0.1.0",
    });
  });

  it("ignores reject when unmounted during load", async () => {
    let rejectPromise!: (reason: unknown) => void;
    const promise = new Promise((_, reject) => {
      rejectPromise = reject;
    });
    vi.spyOn(halberdLib, "loadHalberd").mockReturnValue(
      promise as unknown as ReturnType<typeof halberdLib.loadHalberd>,
    );

    const { unmount } = render(React.createElement(Playground));
    unmount();
    rejectPromise(new Error("Unmounted error"));
  });
});
