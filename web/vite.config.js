import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
// Build output lands in the Go embed dir so `go build` ships the SPA.
// Dev server proxies API calls to the Go backend.
export default defineConfig({
    plugins: [react()],
    build: {
        outDir: "../internal/httpapi/static",
        emptyOutDir: true,
    },
    server: {
        port: 5173,
        proxy: {
            "/api": "http://localhost:8080",
            "/healthz": "http://localhost:8080",
        },
    },
});
