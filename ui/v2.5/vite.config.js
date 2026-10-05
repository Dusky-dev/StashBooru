import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tsconfigPaths from "vite-tsconfig-paths";
import viteCompression from "vite-plugin-compression";

const sourcemap = process.env.VITE_APP_SOURCEMAPS === "true";

// https://vitejs.dev/config/
export default defineConfig(() => {
  const plugins = [
    react({
      babel: {
        compact: true,
      },
    }),
    tsconfigPaths(),
    viteCompression({
      algorithm: "gzip",
      deleteOriginFile: true,
      threshold: 0,
      filter: /\.(js|json|css|svg|md)$/i,
    }),
  ];

  return {
    base: "",
    define: {
      "import.meta.env.VITE_APP_STASHBOORU_VERSION": JSON.stringify(
        readFileSync(
          new URL(
            "../../internal/build/stashbooru-version.txt",
            import.meta.url
          ),
          "utf8"
        ).trim()
      ),
    },
    build: {
      outDir: "build",
      sourcemap: sourcemap,
      reportCompressedSize: false,
    },
    optimizeDeps: {
      entries: "src/index.tsx",
    },
    server: {
      port: 3000,
      cors: false,
    },
    publicDir: "public",
    assetsInclude: ["**/*.md"],
    plugins,
  };
});
