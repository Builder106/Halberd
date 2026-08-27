import { describe, it, expect, vi, beforeEach } from "vitest";
import { parsePacks, parseDecision, parseResponseResult } from "./halberd";

describe("parsePacks", () => {
  it("puts mcp-server-postgres first", () => {
    const input = JSON.stringify([
      { name: "mcp-server-git", server: "stdio", responseFilters: false },
      { name: "mcp-server-postgres", server: "stdio", responseFilters: true },
      { name: "halberd-honeypot", server: "stdio", responseFilters: true },
    ]);
    const result = parsePacks(input);
    expect(result[0].name).toBe("mcp-server-postgres");
  });

  it("puts halberd-honeypot last", () => {
    const input = JSON.stringify([
      { name: "halberd-honeypot", server: "stdio", responseFilters: true },
      { name: "mcp-server-git", server: "stdio", responseFilters: false },
    ]);
    const result = parsePacks(input);
    expect(result[result.length - 1].name).toBe("halberd-honeypot");
  });

  it("sorts remaining packs alphabetically between postgres and honeypot", () => {
    const input = JSON.stringify([
      { name: "mcp-server-z", server: "stdio", responseFilters: false },
      { name: "mcp-server-a", server: "stdio", responseFilters: false },
      { name: "mcp-server-postgres", server: "stdio", responseFilters: true },
      { name: "halberd-honeypot", server: "stdio", responseFilters: true },
    ]);
    const result = parsePacks(input);
    expect(result.map((p) => p.name)).toEqual([
      "mcp-server-postgres",
      "mcp-server-a",
      "mcp-server-z",
      "halberd-honeypot",
    ]);
  });

  it("returns an empty array for an empty list", () => {
    expect(parsePacks("[]")).toEqual([]);
  });
});

describe("parseDecision", () => {
  it("parses a blocked decision", () => {
    const payload = JSON.stringify({
      Blocked: true,
      Violations: [
        { Category: "sql", Tool: "query", Field: "sql", Rule: "drop", Detail: "DROP TABLE" },
      ],
    });
    const d = parseDecision(payload);
    expect(d.Blocked).toBe(true);
    expect(d.Violations).toHaveLength(1);
    expect(d.Violations![0].Rule).toBe("drop");
  });

  it("parses an allowed decision with null violations", () => {
    const payload = JSON.stringify({ Blocked: false, Violations: null });
    const d = parseDecision(payload);
    expect(d.Blocked).toBe(false);
    expect(d.Violations).toBeNull();
  });

  it("throws when the response contains an error field", () => {
    expect(() => parseDecision(JSON.stringify({ error: "pack not found" }))).toThrow(
      "pack not found",
    );
  });
});

describe("parseResponseResult", () => {
  it("parses a modified result with detections", () => {
    const payload = JSON.stringify({
      modified: true,
      payload: '{"redacted":true}',
      detections: [{ kind: "aws-key", path: "result.content[0].text" }],
    });
    const r = parseResponseResult(payload);
    expect(r.modified).toBe(true);
    expect(r.detections).toHaveLength(1);
    expect(r.detections![0].kind).toBe("aws-key");
  });

  it("parses an unmodified result with null detections", () => {
    const payload = JSON.stringify({ modified: false, payload: '{"ok":true}', detections: null });
    const r = parseResponseResult(payload);
    expect(r.modified).toBe(false);
    expect(r.detections).toBeNull();
  });

  it("throws when the response contains an error field", () => {
    expect(() => parseResponseResult(JSON.stringify({ error: "engine error" }))).toThrow(
      "engine error",
    );
  });
});

describe("loadHalberd", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it("rejects when window is undefined", async () => {
    const originalWindow = globalThis.window;
    // @ts-expect-error intentionally simulating non-browser environment
    delete (globalThis as { window?: unknown }).window;
    try {
      const { loadHalberd: load } = await import("./halberd");
      await expect(load()).rejects.toThrow("halberd-wasm only loads in the browser");
    } finally {
      globalThis.window = originalWindow;
    }
  });

  it("rejects when wasm_exec.js script fails to load", async () => {
    const originalGo = globalThis.Go;
    // @ts-expect-error cleanup Go
    delete (globalThis as { Go?: unknown }).Go;

    vi.spyOn(document.head, "appendChild").mockImplementation((node: Node) => {
      const script = node as HTMLScriptElement;
      setTimeout(() => script.onerror?.(new Event("error")), 0);
      return node;
    });

    try {
      const { loadHalberd: load } = await import("./halberd");
      await expect(load()).rejects.toThrow("wasm_exec.js failed to load");
    } finally {
      globalThis.Go = originalGo;
    }
  });

  it("throws when Go runtime did not initialize after script load", async () => {
    const originalGo = globalThis.Go;
    // @ts-expect-error cleanup Go
    delete (globalThis as { Go?: unknown }).Go;

    vi.spyOn(document.head, "appendChild").mockImplementation((node: Node) => {
      const script = node as HTMLScriptElement;
      setTimeout(() => script.onload?.(new Event("load")), 0);
      return node;
    });

    try {
      const { loadHalberd: load } = await import("./halberd");
      await expect(load()).rejects.toThrow("Go runtime did not initialize");
    } finally {
      globalThis.Go = originalGo;
    }
  });

  it("successfully loads, instantiates wasm, starts go run, and caches loading promise", async () => {
    const originalGo = globalThis.Go;
    const originalHalberd = globalThis.halberd;
    const originalFetch = globalThis.fetch;
    const originalInstantiate = WebAssembly.instantiateStreaming;

    const mockRun = vi.fn();
    class MockGo {
      importObject = { env: {} };
      run = mockRun;
    }

    const mockHalberd = {
      packs: vi.fn(() => "[]"),
      evaluateRequest: vi.fn(),
      evaluateResponse: vi.fn(),
      version: "0.1.0",
    };

    // @ts-expect-error clear Go
    delete (globalThis as { Go?: unknown }).Go;
    // @ts-expect-error clear halberd
    delete (globalThis as { halberd?: unknown }).halberd;

    vi.spyOn(document.head, "appendChild").mockImplementation((node: Node) => {
      const script = node as HTMLScriptElement;
      globalThis.Go = MockGo as unknown as typeof Go;
      setTimeout(() => script.onload?.(new Event("load")), 0);
      return node;
    });

    const mockInstance = { exports: {} } as unknown as WebAssembly.Instance;
    vi.spyOn(WebAssembly, "instantiateStreaming").mockImplementation(async () => {
      setTimeout(() => {
        globalThis.halberd = mockHalberd;
      }, 10);
      return { instance: mockInstance } as unknown as WebAssembly.WebAssemblyInstantiatedSource;
    });

    globalThis.fetch = vi.fn().mockResolvedValue(new Response()) as unknown as typeof fetch;

    try {
      const { loadHalberd: load } = await import("./halberd");
      const p1 = load();
      const p2 = load();
      expect(p1).toBe(p2);
      const result = await p1;
      expect(result).toBe(mockHalberd);
      expect(mockRun).toHaveBeenCalledWith(mockInstance);
    } finally {
      globalThis.Go = originalGo;
      globalThis.halberd = originalHalberd;
      globalThis.fetch = originalFetch;
      WebAssembly.instantiateStreaming = originalInstantiate;
    }
  });

  it("loads when Go is already defined on globalThis", async () => {
    const originalGo = globalThis.Go;
    const originalHalberd = globalThis.halberd;
    const originalFetch = globalThis.fetch;
    const originalInstantiate = WebAssembly.instantiateStreaming;

    const mockHalberd = {
      packs: vi.fn(() => "[]"),
      evaluateRequest: vi.fn(),
      evaluateResponse: vi.fn(),
      version: "0.1.0",
    };

    const mockRun = vi.fn().mockImplementation(() => {
      globalThis.halberd = mockHalberd;
    });
    class MockGo {
      importObject = { env: {} };
      run = mockRun;
    }

    globalThis.Go = MockGo as unknown as typeof Go;
    // @ts-expect-error clear halberd
    delete (globalThis as { halberd?: unknown }).halberd;

    const mockInstance = { exports: {} } as unknown as WebAssembly.Instance;
    vi.spyOn(WebAssembly, "instantiateStreaming").mockResolvedValue({
      instance: mockInstance,
    } as unknown as WebAssembly.WebAssemblyInstantiatedSource);
    globalThis.fetch = vi.fn().mockResolvedValue(new Response()) as unknown as typeof fetch;

    try {
      const { loadHalberd: load } = await import("./halberd");
      const result = await load();
      expect(result).toBe(mockHalberd);
      expect(mockRun).toHaveBeenCalledWith(mockInstance);
    } finally {
      globalThis.Go = originalGo;
      globalThis.halberd = originalHalberd;
      globalThis.fetch = originalFetch;
      WebAssembly.instantiateStreaming = originalInstantiate;
    }
  });

  it("throws if halberd global is not registered within 100 ticks", async () => {
    const originalGo = globalThis.Go;
    const originalHalberd = globalThis.halberd;
    const originalFetch = globalThis.fetch;
    const originalInstantiate = WebAssembly.instantiateStreaming;

    const mockRun = vi.fn();
    class MockGo {
      importObject = { env: {} };
      run = mockRun;
    }

    globalThis.Go = MockGo as unknown as typeof Go;
    // @ts-expect-error clear halberd
    delete (globalThis as { halberd?: unknown }).halberd;

    const mockInstance = { exports: {} } as unknown as WebAssembly.Instance;
    vi.spyOn(WebAssembly, "instantiateStreaming").mockResolvedValue({
      instance: mockInstance,
    } as unknown as WebAssembly.WebAssemblyInstantiatedSource);
    globalThis.fetch = vi.fn().mockResolvedValue(new Response()) as unknown as typeof fetch;

    try {
      const { loadHalberd: load } = await import("./halberd");
      await expect(load()).rejects.toThrow(
        "halberd-wasm did not register the global within 1s",
      );
    } finally {
      globalThis.Go = originalGo;
      globalThis.halberd = originalHalberd;
      globalThis.fetch = originalFetch;
      WebAssembly.instantiateStreaming = originalInstantiate;
    }
  });
});

