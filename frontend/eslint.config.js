import js from "@eslint/js";
import svelte from "eslint-plugin-svelte";
import globals from "globals";
import ts from "typescript-eslint";

import svelteConfig from "./svelte.config.js";

const network = "The UI calls only bound Go services (frontend/bindings). It never opens a URL of its own.";

export default ts.config(
  { ignores: ["bindings/", "dist/", "node_modules/", "playwright-report/", "test-results/"] },
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs.recommended,
  {
    languageOptions: { globals: { ...globals.browser } },
  },
  {
    files: ["**/*.svelte", "**/*.svelte.ts"],
    languageOptions: {
      parserOptions: { parser: ts.parser, extraFileExtensions: [".svelte"], svelteConfig },
    },
  },
  {
    files: ["src/**"],
    rules: {
      "no-restricted-globals": [
        "error",
        { name: "fetch", message: network },
        { name: "XMLHttpRequest", message: network },
        { name: "WebSocket", message: network },
        { name: "EventSource", message: network },
      ],
      "no-restricted-properties": [
        "error",
        { object: "window", property: "fetch", message: network },
        { object: "globalThis", property: "fetch", message: network },
      ],
    },
  },
  {
    files: ["*.config.{js,ts}", "e2e/**"],
    languageOptions: { globals: { ...globals.node } },
  },
);
