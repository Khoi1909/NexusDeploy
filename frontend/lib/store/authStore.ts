import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

interface User {
  id: string;
  username: string;
  email: string;
  avatar_url: string;
  plan: "standard" | "premium";
}

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;

  // Actions
  setTokens: (accessToken: string, refreshToken: string) => void;
  setUser: (user: User) => void;
  logout: () => void;
  setLoading: (loading: boolean) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      isAuthenticated: false,
      isLoading: true,

      setTokens: (accessToken, refreshToken) =>
        set({
          accessToken,
          refreshToken,
          isAuthenticated: true,
          isLoading: false,
          user: null, // Clear previous user data
        }),

      setUser: (user) =>
        set({
          user,
        }),

      logout: () => {
        // Clear all auth state
        set({
          accessToken: null,
          refreshToken: null,
          user: null,
          isAuthenticated: false,
          isLoading: false,
        });
        // Clear any legacy localStorage items if they exist
        if (typeof window !== "undefined") {
          localStorage.removeItem("nexus_token");
          localStorage.removeItem("nexus_user");
        }
      },

      setLoading: (isLoading) =>
        set({
          isLoading,
        }),
    }),
    {
      name: "nexusdeploy-auth",
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state, error) => {
        // Set isLoading to false after rehydration completes
        if (state) {
          state.isLoading = false;
        }
        if (error) {
          console.error("Auth store rehydration error:", error);
        }
      },
    }
  )
);

