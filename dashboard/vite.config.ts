import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// During development the dashboard proxies /api to the SystemCheck server.
// The server uses a self-signed dev cert, so TLS verification is disabled for
// the dev proxy only.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "https://127.0.0.1:8443",
        changeOrigin: true,
        secure: false,
      },
    },
  },
});
