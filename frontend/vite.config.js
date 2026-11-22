import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";
// __dirname doesn't exist in native ESM (package.json has "type": "module"),
// hence the fileURLToPath dance instead.
var currentDir = path.dirname(fileURLToPath(import.meta.url));
// STELLAREI never runs a dev server against a live Wails backend inside
// this sandbox (see README.md - no Wails toolchain here), but `vite dev`
// still works standalone for pure UI iteration. `vite build` produces
// dist/, which main.go embeds directly into the Go binary - there is no
// separate frontend deployment step in the shipped app.
export default defineConfig({
    plugins: [react()],
    base: "./", // asset URLs must be relative: they're served from an
    // embedded filesystem via Wails' AssetFileServerFS, not from a domain root.
    resolve: {
        alias: {
            "@": path.resolve(currentDir, "src"),
        },
    },
    build: {
        outDir: "dist",
        emptyOutDir: true,
    },
});
