/// <reference types="vitest/config" />

import { fileURLToPath, URL } from "node:url";
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

const embeddedOutDir = resolve(import.meta.dirname, "../internal/webui/dist");

const keepEmbedSentinel = {
  name: "keep-embed-sentinel",
  async closeBundle() {
    await writeFile(resolve(embeddedOutDir, "placeholder.txt"), "singbox-web embedded UI build sentinel\n", "utf8");
  },
};

export default defineConfig({
  plugins: [vue(), keepEmbedSentinel],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: embeddedOutDir,
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
  },
});
