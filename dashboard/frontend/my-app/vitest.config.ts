import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "coverage",
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/main.tsx",
        "src/vite-env.d.ts",
        "src/test/**",
        "src/components/ui/**",
        "src/lib/mocks/fixtures/**",
        "src/**/*.test.{ts,tsx}",
      ],
      // 防回退基线（2026-08-21 实测）；目标抬升见 issue #142/#143
      thresholds: {
        global: {
          statements: 24,
          branches: 19,
          functions: 21,
          lines: 24,
        },
      },
    },
  },
});
