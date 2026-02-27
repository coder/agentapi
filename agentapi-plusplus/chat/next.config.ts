import type { NextConfig } from "next";
let basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "/chat";
if (basePath.endsWith("/")) {
  basePath = basePath.slice(0, -1);
}

const nextConfig: NextConfig = {
  // Enable static exports
  output: "export",

  // Disable image optimization since it's not supported in static exports
  images: {
    unoptimized: true,
  },

  // Configure base path for GitHub Pages (repo/chat)
  basePath,

  // Configure asset prefix for GitHub Pages - helps with static asset loading
  assetPrefix: `${basePath}/`,

  // Configure trailing slashes (recommended for static exports)
  trailingSlash: true,
};

export default nextConfig;
