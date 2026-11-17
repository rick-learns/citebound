import { defineConfig } from "vite";

export default defineConfig({
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    // The example-question test reads the recorded answers (src/render/form.test.ts).
    fs: { allow: [".", "../evals/recordings"] },
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
});
