"use client";

import { ReactNode } from "react";
import { useAuthStore } from "@/lib/store/authStore";
import { Loader2 } from "lucide-react";

interface AuthProviderProps {
  children: ReactNode;
}

/**
 * AuthProvider ensures auth state is rehydrated before rendering children.
 * Zustand persist middleware handles the actual localStorage sync,
 * but we need to wait for isLoading to be false before rendering.
 */
export function AuthProvider({ children }: AuthProviderProps) {
  const { isLoading } = useAuthStore();

  // Wait for auth state to be rehydrated from localStorage
  // This prevents flash of wrong UI (e.g., showing dashboard button when logged out)
  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  return <>{children}</>;
}

