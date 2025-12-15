import { apiClient } from "./client";

export interface Build {
  id: string;
  project_id: string;
  commit_sha: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}

export interface BuildStep {
  id: string;
  build_id: string;
  step_name: string;
  status: string;
  duration_ms?: number;
}

export interface BuildLog {
  id: number;
  build_id: string;
  timestamp: string;
  log_line: string;
}

export interface BuildDetails extends Build {
  steps: BuildStep[];
}

export interface BuildLogsResponse {
  logs: BuildLog[];
  has_more: boolean;
}

export interface AnalysisResult {
  analysis: string;
  suggestions: string[];
  cached: boolean;
}

export const buildsApi = {
  // List builds for a project with pagination
  listBuilds: async (
    token: string,
    projectId: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<{ builds: Build[]; total: number }> => {
    const response = await apiClient.get<{ builds: Build[]; total: number }>(
      `/api/projects/${projectId}/builds?page=${page}&page_size=${pageSize}`,
      { token }
    );
    return response;
  },

  // Get single build with details
  getBuild: async (token: string, buildId: string): Promise<BuildDetails> => {
    const response = await apiClient.get<{ build: Build; steps: BuildStep[] }>(
      `/api/builds/${buildId}`,
      { token }
    );
    return {
      ...response.build,
      steps: response.steps || [],
    };
  },

  // Trigger a new build
  triggerBuild: async (
    token: string,
    projectId: string,
    commitSha?: string,
    branch?: string,
    repoUrl?: string
  ): Promise<Build> => {
    const response = await apiClient.post<{ build: Build }>(
      `/api/projects/${projectId}/builds`,
      {
        commit_sha: commitSha,
        branch: branch,
        repo_url: repoUrl,
      },
      { token }
    );
    return response.build;
  },

  // Get build logs with pagination
  getBuildLogs: async (
    token: string,
    buildId: string,
    limit: number = 500,
    afterId?: number
  ): Promise<BuildLogsResponse> => {
    const params = new URLSearchParams({
      limit: limit.toString(),
    });
    if (afterId !== undefined) {
      params.append("after_id", afterId.toString());
    }

    const response = await apiClient.get<BuildLogsResponse>(
      `/api/builds/${buildId}/logs?${params.toString()}`,
      { token }
    );
    return response;
  },

  // Clear all build logs for a project
  clearBuildLogs: async (
    token: string,
    projectId: string
  ): Promise<void> => {
    await apiClient.delete(
      `/api/projects/${projectId}/builds/logs`,
      { token }
    );
  },

  // Analyze build errors using AI
  analyzeBuild: async (
    token: string,
    buildId: string
  ): Promise<AnalysisResult> => {
    const response = await apiClient.post<AnalysisResult>(
      `/api/builds/${buildId}/analyze`,
      {},
      { token }
    );
    return response;
  },
};

