import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// During development the dashboard proxies /api to the SystemCheck server.
// The server uses a self-signed dev cert, so TLS verification is disabled for
// the dev proxy only.
//
// The server's session cookie is set with `Secure`, which browsers refuse to
// store over the plain-HTTP dev server (http://localhost:5173). For DEV ONLY we
// strip the `Secure` attribute from Set-Cookie as it passes back through the
// proxy so the session sticks. Never do this in production (serve over HTTPS).
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "https://127.0.0.1:8443",
        changeOrigin: true,
        secure: false,
        configure: (proxy) => {
          proxy.on("proxyRes", (proxyRes) => {
            const sc = proxyRes.headers["set-cookie"];
            if (sc) {
              proxyRes.headers["set-cookie"] = sc.map((c) =>
                c.replace(/;\s*Secure/gi, "")
              );
            }
          });
        },
      },
    },
  },
});
