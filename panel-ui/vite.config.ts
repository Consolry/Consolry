import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The build lands inside the Go panel package, which embeds it into the binary.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/panel/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8700", ws: true },
    },
  },
});
