import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "node",
    globals: true,
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      include: [
        "src/components/Playground.tsx",
        "src/components/Hero.tsx",
        "src/components/ThreatModel.tsx",
        "src/components/WaxSeal.tsx",
        "src/components/Crest.tsx",
        "src/lib/presets.ts",
        "src/lib/halberd.ts",
      ],
      exclude: ["src/**/*.test.{ts,tsx}"],
      thresholds: {
        statements: 100,
        branches: 100,
        functions: 100,
        lines: 100,
      },
    },
  },
});

