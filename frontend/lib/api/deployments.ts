import { apiClient } from "./client";

export interface Deployment {
  id: string;
  project_id: string;
  container_id: string;
  status: string;
  public_url: string;
}

export const deploymentsApi = {
  // Deploy from latest successful build
  deploy: async (token: string, projectId: string): Promise<Deployment> => {
    const response = await apiClient.post<{ deployment: Deployment }>(
      `/api/projects/${projectId}/deploy`,
      {},
      { token }
    );
    return response.deployment;
  },

  // Stop deployment
  stopDeployment: async (
    token: string,
    projectId: string
  ): Promise<{ success: boolean }> => {
    const response = await apiClient.post<{ success: boolean }>(
      `/api/projects/${projectId}/stop`,
      {},
      { token }
    );
    return response;
  },

  // Restart deployment
  restartDeployment: async (
    token: string,
    projectId: string
  ): Promise<{ success: boolean; container_id: string }> => {
    const response = await apiClient.post<{
      success: boolean;
      container_id: string;
    }>(`/api/projects/${projectId}/restart`, {}, { token });
    return response;
  },

  // Get deployment status
  getDeploymentStatus: async (
    token: string,
    projectId: string
  ): Promise<Deployment | null> => {
    const response = await apiClient.get<{ deployment: Deployment | null }>(
      `/api/projects/${projectId}/deployment`,
      { token }
    );
    return response.deployment || null;
  },
};

