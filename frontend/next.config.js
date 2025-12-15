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
    // WebSocket URL - use relative path or actual domain in production
    // Default empty, will be set via environment variable
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || '',
  },
  
  // API proxy to backend (development + production)
  // Server-side rewrites use internal docker network service name
  async rewrites() {
    // Use internal service name for server-side rewrites (docker network)
    // NEXT_PUBLIC_API_URL is for client-side, not used here
    const apiUrl = process.env.API_URL || 'http://api-gateway:8000';
    return [
      {
        source: '/api/:path*',
        destination: `${apiUrl}/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
