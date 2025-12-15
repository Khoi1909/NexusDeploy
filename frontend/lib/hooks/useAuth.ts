"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/lib/store/authStore";

const publicRoutes = ["/", "/login", "/auth/callback"];

export function useAuth() {
  const router = useRouter();
  const pathname = usePathname();
  const { accessToken, user, logout } = useAuthStore();

  const isAuthenticated = !!accessToken;
  const isPublicRoute = publicRoutes.includes(pathname);

  useEffect(() => {
    // If not authenticated and trying to access protected route, redirect to login
    if (!isAuthenticated && !isPublicRoute) {
      router.push("/login");
    }
  }, [isAuthenticated, isPublicRoute, router]);

  const signOut = () => {
    logout();
    router.push("/");
  };

  return {
    user,
    token: accessToken,
    isAuthenticated,
    isLoading: false,
    signOut,
  };
}

export function useRequireAuth() {
  const router = useRouter();
  const { accessToken, user } = useAuthStore();

  useEffect(() => {
    if (!accessToken) {
      router.push("/login");
    }
  }, [accessToken, router]);

  return { user, token: accessToken, isAuthenticated: !!accessToken };
}

