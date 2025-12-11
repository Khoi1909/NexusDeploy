"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuthStore } from "@/lib/store/authStore";
import { Loader2, XCircle } from "lucide-react";

type CallbackStatus = "loading" | "error";

export default function AuthCallbackPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setTokens, setUser } = useAuthStore();
  const [status, setStatus] = useState<CallbackStatus>("loading");
  const [errorMessage, setErrorMessage] = useState<string>("");

  const decodeJwt = (token: string): Record<string, any> | null => {
    try {
      const payload = token.split(".")[1];
      const decoded = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
      return JSON.parse(decoded);
    } catch (e) {
      return null;
    }
  };

  useEffect(() => {
    // Always read from current URL (avoids hydration mismatch)
    const params =
      typeof window !== "undefined"
        ? new URL(window.location.href).searchParams
        : new URLSearchParams(searchParams.toString());

    const err = params.get("error");
    if (err) {
      setStatus("error");
      setErrorMessage(params.get("message") || "Authentication failed");
      if (typeof window !== "undefined") {
        window.location.replace(`/login?error=${encodeURIComponent(err)}`);
      } else {
        router.replace("/login");
      }
      return;
    }

    const accessToken = params.get("access_token");
    const userId = params.get("user_id");
    const usernameParam = params.get("username");
    const planParam = params.get("plan");
    const refreshToken = params.get("refresh_token") || "";

    if (!accessToken) {
      setStatus("error");
      setErrorMessage("No access token received");
      if (typeof window !== "undefined") {
        window.location.replace("/login?error=missing_token");
      } else {
        router.replace("/login");
      }
      return;
    }

    const jwtData = decodeJwt(accessToken);
    const username =
      usernameParam ||
      (jwtData && (jwtData.username || jwtData.user || jwtData.name)) ||
      "GitHubUser";
    const plan =
      (planParam as "standard" | "premium") ||
      (jwtData && jwtData.plan) ||
      "standard";
    const avatar =
      (jwtData && (jwtData.avatar_url || jwtData.avatar || jwtData.picture)) || "";

    // Persist auth
    setTokens(accessToken, refreshToken);
    setUser({
      id: userId || "",
      username,
      email: "",
      avatar_url: avatar,
      plan,
    });

    // Hard redirect to dashboard to avoid stuck UI
    if (typeof window !== "undefined") {
      window.location.assign("/dashboard");
    } else {
      router.replace("/dashboard");
    }
  }, [searchParams, setTokens, setUser, router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-6 text-center">
        {status === "loading" && (
          <>
            <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-foreground">Completing sign in...</h2>
              <p className="mt-2 text-surface-400">Please wait while we authenticate you.</p>
            </div>
          </>
        )}

        {status === "error" && (
          <>
            <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-accent-rose/10">
              <XCircle className="h-8 w-8 text-accent-rose" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-foreground">Authentication failed</h2>
              <p className="mt-2 text-surface-400">{errorMessage}</p>
            </div>
            <a
              href="/"
              className="mt-4 rounded-lg bg-surface-800 px-6 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-surface-700"
            >
              Return to Homepage
            </a>
          </>
        )}
      </div>
    </div>
  );
}

