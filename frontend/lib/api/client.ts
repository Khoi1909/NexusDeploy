import { useAuthStore } from "@/lib/store/authStore";

// Use relative path if NEXT_PUBLIC_API_URL starts with /, otherwise use absolute URL
// Default to relative path /api for production (routed through Traefik)
const apiUrl = process.env.NEXT_PUBLIC_API_URL || "/api";
const API_BASE_URL = apiUrl.startsWith("/") ? "" : apiUrl;

interface RequestOptions extends RequestInit {
  token?: string;
}

interface ApiError {
  error: string;
  message: string;
  status?: number;
}

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestOptions = {}
  ): Promise<T> {
    const { token, ...fetchOptions } = options;

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    };

    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    // Normalize endpoint: ensure it starts with / if baseUrl is empty
    // With NEXT_PUBLIC_API_URL=https://khqi.io.vn, baseUrl = "https://khqi.io.vn"
    // All endpoints should include /api/ prefix to match Traefik routing
    let normalizedEndpoint = endpoint;
    if (this.baseUrl === "" && !endpoint.startsWith("/")) {
      normalizedEndpoint = `/${endpoint}`;
    }
    
    // If baseUrl doesn't end with /api, ensure endpoint has /api prefix
    // This handles:
    // - baseUrl = "https://khqi.io.vn" and endpoint = "/api/projects" → keep as is
    if (this.baseUrl !== "" && !this.baseUrl.endsWith("/api") && !normalizedEndpoint.startsWith("/api")) {
      normalizedEndpoint = `/api${normalizedEndpoint}`;
    }
    
    // Remove duplicate /api prefix if baseUrl already ends with /api
    if (this.baseUrl !== "" && this.baseUrl.endsWith("/api") && normalizedEndpoint.startsWith("/api/")) {
      normalizedEndpoint = normalizedEndpoint.substring(4); // Remove "/api"
    }

    // Combine baseUrl and endpoint, ensuring no double slashes
    const url = `${this.baseUrl}${normalizedEndpoint}`.replace(/([^:]\/)\/+/g, "$1");

    const response = await fetch(url, {
      ...fetchOptions,
      headers,
    });

    if (!response.ok) {
      // Handle 401/403 - unauthorized/forbidden (invalid or expired token)
      if (response.status === 401 || response.status === 403) {
        // Clear auth state immediately
        const { logout } = useAuthStore.getState();
        logout();
        
        // Redirect to homepage - use window.location for hard redirect
        // This prevents any further execution and immediately kicks user out
        if (typeof window !== "undefined") {
          window.location.href = "/";
        }
        
        // Throw error to prevent further execution
        throw new Error("Authentication required");
      }

      // Handle other errors
      let errorMessage = `HTTP error ${response.status}`;
      let errorCode = "unknown_error";
      
      try {
        const errorData: ApiError = await response.json();
        errorMessage = errorData.message || errorData.error || errorMessage;
        errorCode = errorData.error || errorCode;
      } catch {
        // If response is not JSON, try to get text
        try {
          const text = await response.text();
          if (text) {
            errorMessage = text;
          }
        } catch {
          // If all else fails, use default message
          errorMessage = `HTTP error ${response.status}: ${response.statusText}`;
        }
      }
      
      const error = new Error(errorMessage);
      (error as any).code = errorCode;
      (error as any).status = response.status;
      throw error;
    }

    // Handle empty responses
    const text = await response.text();
    if (!text) return {} as T;

    return JSON.parse(text);
  }

  get<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: "GET" });
  }

  post<T>(endpoint: string, data?: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  put<T>(endpoint: string, data?: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: "PUT",
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  delete<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: "DELETE" });
  }
}

export const apiClient = new ApiClient(API_BASE_URL);
export { API_BASE_URL };

