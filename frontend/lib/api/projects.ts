import { apiClient } from "./client";
import { Project } from "@/lib/store/projectStore";

interface Repository {
  id: number;
  name: string;
  full_name: string;
  description: string;
  html_url: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
}

export interface CreateProjectRequest {
  name: string;
  repo_url: string;
  branch?: string;
  preset: string;
  build_command?: string;
  start_command?: string;
  port?: number;
}

interface Build {
  id: string;
  project_id: string;
  commit_sha: string;
  status: string;
  started_at: string;
  finished_at?: string;
}

interface Secret {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export const projectsApi = {
  // List all projects
  listProjects: async (token: string): Promise<Project[]> => {
    const response = await apiClient.get<{ projects: Project[] }>("/api/projects", { token });
    return response.projects || [];
  },

  // Get single project
  getProject: async (token: string, projectId: string): Promise<Project> => {
    const response = await apiClient.get<{ project: Project }>(`/api/projects/${projectId}`, { token });
    return response.project;
  },

  // Create new project
  createProject: async (
    token: string,
    data: CreateProjectRequest
  ): Promise<Project> => {
    const response = await apiClient.post<{ project: Project }>("/api/projects", data, { token });
    return response.project;
  },

  // Delete project
  deleteProject: async (token: string, projectId: string): Promise<void> => {
    return apiClient.delete(`/api/projects/${projectId}`, { token });
  },

  // List GitHub repositories
  listRepositories: async (token: string): Promise<Repository[]> => {
    const response = await apiClient.get<{ repositories: Repository[] }>(
      "/api/repos",
      { token }
    );
    return response.repositories || [];
  },

  // Get project builds
  getBuilds: async (token: string, projectId: string): Promise<Build[]> => {
    const response = await apiClient.get<{ builds: Build[] }>(
      `/api/projects/${projectId}/builds`,
      { token }
    );
    return response.builds || [];
  },

  // Trigger new build
  triggerBuild: async (token: string, projectId: string): Promise<Build> => {
    const response = await apiClient.post<{ build: Build }>(`/api/projects/${projectId}/builds`, {}, { token });
    return response.build;
  },

  // List secrets
  listSecrets: async (token: string, projectId: string): Promise<Secret[]> => {
    const response = await apiClient.get<{ secrets: Secret[] }>(
      `/api/projects/${projectId}/secrets`,
      { token }
    );
    return response.secrets || [];
  },

  // Add secret
  addSecret: async (
    token: string,
    projectId: string,
    name: string,
    value: string
  ): Promise<Secret> => {
    const response = await apiClient.post<{ secret: Secret }>(
      `/api/projects/${projectId}/secrets`,
      { name, value },
      { token }
    );
    return response.secret;
  },

  // Delete secret
  deleteSecret: async (
    token: string,
    projectId: string,
    secretId: string
  ): Promise<void> => {
    return apiClient.delete(`/api/projects/${projectId}/secrets/${secretId}`, {
      token,
    });
  },
};

// Backward compatibility alias
export const projectApi = projectsApi;

