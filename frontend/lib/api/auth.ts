import { apiClient, API_BASE_URL } from "./client";

interface LoginResponse {
  access_token: string;
  refresh_token: string;
  expires_at_unix: number;
  user_id: string;
  plan: string;
}

interface RefreshResponse {
  access_token: string;
  refresh_token: string;
  expires_at_unix: number;
}

interface UserInfo {
  id: string;
  username: string;
  email: string;
  avatar_url: string;
  plan: "standard" | "premium";
}

export const authApi = {
  // Redirect to GitHub login
  loginWithGitHub: () => {
    // With NEXT_PUBLIC_API_URL=https://khqi.io.vn, API_BASE_URL = "https://khqi.io.vn"
    // Need to add /api prefix for Traefik routing
    const base = API_BASE_URL || "";
    // Add /api if baseUrl doesn't end with /api
    const apiPrefix = base.endsWith("/api") ? "" : "/api";
    window.location.href = `${base}${apiPrefix}/auth/github/login`;
  },

  // Handle OAuth callback
  handleCallback: async (code: string): Promise<LoginResponse> => {
    return apiClient.get<LoginResponse>(`/api/auth/github/callback?code=${code}`);
  },

  // Refresh tokens
  refreshToken: async (refreshToken: string): Promise<RefreshResponse> => {
    return apiClient.post<RefreshResponse>("/api/auth/refresh", {
      refresh_token: refreshToken,
    });
  },

  // Get current user info
  getCurrentUser: async (token: string): Promise<UserInfo> => {
    return apiClient.get<UserInfo>("/api/auth/me", { token });
  },

  // Logout
  logout: async (token: string): Promise<void> => {
    return apiClient.post("/api/auth/logout", {}, { token });
  },

  // Update plan
  updatePlan: async (token: string, plan: "standard" | "premium"): Promise<{ success: boolean; message: string; plan: string }> => {
    return apiClient.put("/api/user/plan", { plan }, { token });
  },
};

