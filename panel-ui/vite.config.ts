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
      // CONSOLRY_DEV_PANEL and CONSOLRY_DEV_COOKIE let a dev server point at another panel
      // with a ready-made session, which is how the interface is screenshotted.
      "/api": {
        target: process.env.CONSOLRY_DEV_PANEL ?? "http://127.0.0.1:8700",
        ws: true,
        headers: process.env.CONSOLRY_DEV_COOKIE ? { Cookie: process.env.CONSOLRY_DEV_COOKIE } : undefined,
      },
    },
  },
});
