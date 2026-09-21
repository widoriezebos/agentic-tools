import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, type ProxyOptions } from "vite";

// Development points at the engine's own server; nothing about the Go side is
// loosened for it, and the override exists for a checkout that runs on another
// port, never for a remote host.
const proxyTarget = process.env.METASYSTEM_UI_PROXY_TARGET ?? "http://127.0.0.1:7878";

// changeOrigin makes the engine see its own Host, and the Origin the browser
// sends to Vite is dropped rather than forwarded as a foreign one; the
// browser's Sec-Fetch-Site: same-origin is forwarded and passes the checks.
// ProxyOptions types configure as (proxy: ProxyServer, options: ProxyOptions),
// and ProxyServer is an EventEmitter whose proxyReq event carries the outgoing
// http.ClientRequest first; both parameters are inferred from that.
const proxied: ProxyOptions = {
  target: proxyTarget,
  changeOrigin: true,
  configure: (proxy) => {
    proxy.on("proxyReq", (request) => {
      request.removeHeader("origin");
    });
  },
};

export default defineConfig({
  base: "/",
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../bundle/dist",
    emptyOutDir: true,
    // No data: URL reaches the page: the policy allows none, so every asset is
    // a file the server hands out under its own path.
    assetsInlineLimit: 0,
    sourcemap: false,
    manifest: false,
    modulePreload: { polyfill: false },
    rolldownOptions: {
      output: {
        entryFileNames: "assets/[name].js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": proxied,
      "/-": proxied,
    },
  },
});
