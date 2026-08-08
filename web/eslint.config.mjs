import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Generated Playwright-BDD test files — not source code
    ".features-gen/**",
    // E2E test files — not source code
    "e2e/**",
    // Config files — not source code
    "*.config.*",
    "*.config.mjs",
    "*.config.ts",
    "eslint.config.mjs",
    // Static assets / generated files / scripts — not source code
    "public/**",
    "scripts/**",
  ]),
]);

export default eslintConfig;
