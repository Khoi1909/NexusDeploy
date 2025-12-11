"use client";

import { useEffect, ReactNode } from "react";
import { useAuthStore } from "@/lib/store/authStore";

interface AuthProviderProps {
  children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const { token, user } = useAuthStore();

  useEffect(() => {
    // Check for token in localStorage on mount (for persistence)
    if (typeof window !== "undefined") {
      const storedToken = localStorage.getItem("nexus_token");
      const storedUser = localStorage.getItem("nexus_user");

      if (storedToken && storedUser && !token) {
        try {
          const parsedUser = JSON.parse(storedUser);
          useAuthStore.getState().login(storedToken, parsedUser);
        } catch (e) {
          // Invalid stored data, clear it
          localStorage.removeItem("nexus_token");
          localStorage.removeItem("nexus_user");
        }
      }
    }
  }, [token]);

  // Persist auth state to localStorage
  useEffect(() => {
    if (typeof window !== "undefined") {
      if (token && user) {
        localStorage.setItem("nexus_token", token);
        localStorage.setItem("nexus_user", JSON.stringify(user));
      } else {
        localStorage.removeItem("nexus_token");
        localStorage.removeItem("nexus_user");
      }
    }
  }, [token, user]);

  return <>{children}</>;
}

