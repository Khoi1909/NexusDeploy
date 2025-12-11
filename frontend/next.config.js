/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  
  // Standalone output for Docker production builds
  output: 'standalone',
  
  // Disable image optimization (no external service)
  images: {
    unoptimized: true,
  },
  
  // Environment variables
  env: {
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8000',
  },
  
  // API proxy to backend (development + production)
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://api-gateway:8000';
    return [
      {
        source: '/api/:path*',
        destination: `${apiUrl}/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
