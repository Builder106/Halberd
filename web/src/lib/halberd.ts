// Thin TypeScript surface over the halberd-wasm globals defined in
// cmd/halberd-wasm/main.go. The WASM module exposes everything as JSON
// strings (the syscall/js bridge is friendliest with primitives), so
// this file parses them into typed values for the UI.

export type PackInfo = {
  name: string;
  server: string;
  responseFilters: boolean;
};

export type Violation = {
  Category: string;
  Tool: string;
  Field: string;
  Rule: string;
  Detail: string;
};

export type Decision = {
  Blocked: boolean;
  Violations: Violation[] | null;
};

export type Detection = {
  kind: string;
  path: string;
};

export type ResponseResult = {
  modified: boolean;
  payload: string;
  detections: Detection[] | null;
};

type HalberdGlobal = {
  packs: () => string;
  evaluateRequest: (pack: string, payload: string) => string;
  evaluateResponse: (pack: string, payload: string) => string;
  version: string;
};

declare global {
  var halberd: HalberdGlobal | undefined;
  var Go: { new (): { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> } } | undefined;
}

let loading: Promise<HalberdGlobal> | null = null;

export function loadHalberd(): Promise<HalberdGlobal> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("halberd-wasm only loads in the browser"));
  }
  if (loading) return loading;

  loading = (async () => {
    // wasm_exec.js sets window.Go as a side effect.
    if (!globalThis.Go) {
      await new Promise<void>((resolve, reject) => {
        const s = document.createElement("script");
        s.src = "/wasm_exec.js";
        s.onload = () => resolve();
        s.onerror = () => reject(new Error("wasm_exec.js failed to load"));
        document.head.appendChild(s);
      });
    }
    const Go = globalThis.Go;
    if (!Go) throw new Error("Go runtime did not initialize");
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch("/halberd.wasm"),
      go.importObject,
    );
    // Don't await — go.run blocks forever (the engine's select{} loop).
    void go.run(instance);
    // Wait a tick for main() to register globalThis.halberd.
    for (let i = 0; i < 100; i++) {
      if (globalThis.halberd) return globalThis.halberd;
      await new Promise((r) => setTimeout(r, 10));
    }
    throw new Error("halberd-wasm did not register the global within 1s");
  })();

  return loading;
}

export function parsePacks(json: string): PackInfo[] {
  const arr = JSON.parse(json) as PackInfo[];
  // Sort: postgres first (most familiar), honeypot last (demo-only).
  const order = (n: string) =>
    n === "mcp-server-postgres"
      ? 0
      : n === "halberd-honeypot"
        ? 99
        : 50;
  return arr.sort((a, b) => order(a.name) - order(b.name) || a.name.localeCompare(b.name));
}

export function parseDecision(json: string): Decision {
  const d = JSON.parse(json) as Decision | { error: string };
  if ("error" in d) throw new Error(d.error);
  return d;
}

export function parseResponseResult(json: string): ResponseResult {
  const r = JSON.parse(json) as ResponseResult | { error: string };
  if ("error" in r) throw new Error(r.error);
  return r;
}
