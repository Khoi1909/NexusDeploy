"use client";

import { useEffect } from "react";
import { GitBranch, Loader2 } from "lucide-react";

export default function LoginPage() {
  useEffect(() => {
    // Redirect to GitHub OAuth with redirect_url parameter
    // Always use /api/auth/github/login for Traefik routing
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "/api";
    const redirectUrl = encodeURIComponent(`${window.location.origin}/auth/callback`);
    // Ensure /api prefix is included
    const baseUrl = apiUrl.startsWith("/") 
      ? `${window.location.origin}${apiUrl}`
      : apiUrl;
    // Add /api if baseUrl doesn't end with /api
    const apiPrefix = baseUrl.endsWith("/api") ? "" : "/api";
    const fullUrl = `${baseUrl}${apiPrefix}/auth/github/login?redirect_url=${redirectUrl}`;
    window.location.href = fullUrl;
  }, []);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-accent-violet shadow-lg shadow-primary/20">
          <GitBranch className="h-8 w-8 text-white" />
        </div>
        <div className="flex items-center gap-2 text-surface-400">
          <Loader2 className="h-5 w-5 animate-spin" />
          <span>Redirecting to GitHub...</span>
        </div>
      </div>
    </div>
  );
}

