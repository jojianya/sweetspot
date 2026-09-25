import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // The app is accessed at http://127.0.0.1:3000 in local dev; without this
  // Next 16 blocks the HMR WebSocket (/_next/hmr) as a cross-origin dev
  // resource, which shows up as failed ws://127.0.0.1:3000/_next/hmr
  // connections in the browser console.
  allowedDevOrigins: ["127.0.0.1", "localhost"],
};

export default nextConfig;
