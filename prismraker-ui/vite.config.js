import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// During dev, proxy API + websocket to prismraker-svc.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8420",
      "/api/stream": { target: "ws://localhost:8420", ws: true },
    },
  },
});
