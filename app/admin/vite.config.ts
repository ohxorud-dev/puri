import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/puri.": {
        target: "http://localhost:8080",
        changeOrigin: false,
      },
    },
  },
});
