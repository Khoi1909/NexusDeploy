"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { DotGrid } from "@/components/ui/DotGrid";
import { AnimatedCard } from "@/components/ui/AnimatedCard";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { useProjectStore } from "@/lib/store/projectStore";
import { useAuthStore } from "@/lib/store/authStore";
import { projectApi } from "@/lib/api/projects";
import { buildsApi, Build } from "@/lib/api/builds";
import { deploymentsApi, Deployment } from "@/lib/api/deployments";
import { useRouter } from "next/navigation";
import {
  Plus,
  GitBranch,
  Clock,
  ExternalLink,
  RefreshCw,
  Activity,
  Server,
  AlertTriangle,
} from "lucide-react";

type StatusType = "running" | "stopped" | "building" | "error" | "pending";

const statusLabels: Record<StatusType, string> = {
  running: "Running",
  stopped: "Stopped",
  building: "Building",
  error: "Error",
  pending: "Pending",
};

export default function DashboardPage() {
  const router = useRouter();
  const { projects, setProjects } = useProjectStore();
  const { accessToken, logout, isLoading: authLoading, isAuthenticated } = useAuthStore();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Store build and deployment status for each project
  const [projectStatuses, setProjectStatuses] = useState<Record<string, { build?: Build; deployment?: Deployment | null }>>({});

  useEffect(() => {
    // Wait for auth store to hydrate from localStorage
    if (authLoading) {
      return;
    }

    // Only redirect if store has finished loading and user is not authenticated
    if (!isAuthenticated && !accessToken) {
      setIsLoading(false);
      router.push("/");
      return;
    }

    // Don't proceed if no token (shouldn't happen if authenticated, but safety check)
    if (!accessToken) {
      return;
    }

    // If we already have projects in store, show them immediately (optimistic UI)
    const hasCachedProjects = projects.length > 0;
    if (hasCachedProjects) {
      setIsLoading(false);
    }

    const fetchProjects = async () => {
      try {
        // Only show loading if we don't have cached projects
        if (!hasCachedProjects) {
          setIsLoading(true);
        }
        setError(null);
        
        const data = await projectApi.listProjects(accessToken);
        setProjects(data);
      } catch (err: any) {
        console.error("Failed to fetch projects:", err);
        const status = (err as any).status;
        const errorMessage = err.message || err.code || "Failed to fetch projects";
        
        // Only logout if we get a 401 from the API (token expired/invalid)
        if (status === 401 || errorMessage.toLowerCase().includes("unauthorized")) {
          logout();
          router.push("/");
          return;
        }
        
        setError(errorMessage);
      } finally {
        setIsLoading(false);
      }
    };

    fetchProjects();
  }, [accessToken, authLoading, isAuthenticated, projects.length, setProjects, logout, router]);

  // Fetch build and deployment status for all projects
  useEffect(() => {
    if (!accessToken || authLoading || !isAuthenticated || isLoading || projects.length === 0) {
      return;
    }

    const fetchProjectStatuses = async () => {
      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;

        const statusPromises = projects.map(async (project) => {
          try {
            // Fetch latest build
            const buildsResp = await buildsApi.listBuilds(currentToken, project.id, 1, 1);
            const latestBuild = buildsResp.builds?.[0];

            // Fetch deployment status
            let deployment: Deployment | null = null;
            try {
              deployment = await deploymentsApi.getDeploymentStatus(currentToken, project.id);
            } catch {
              // Deployment might not exist, that's okay
            }

            return {
              projectId: project.id,
              build: latestBuild,
              deployment,
            };
          } catch (err) {
            console.error(`Failed to fetch status for project ${project.id}:`, err);
            return { projectId: project.id, build: undefined, deployment: null };
          }
        });

        const results = await Promise.all(statusPromises);
        const statusMap: Record<string, { build?: Build; deployment?: Deployment | null }> = {};
        results.forEach((result) => {
          statusMap[result.projectId] = {
            build: result.build,
            deployment: result.deployment,
          };
        });
        setProjectStatuses(statusMap);
      } catch (err: any) {
        console.error("Failed to fetch project statuses:", err);
      }
    };

    fetchProjectStatuses();

    // Poll if any project is building or pending
    const needsPolling = projects.some((p) => {
      const status = projectStatuses[p.id];
      const projectStatus = (p as any).status as string;
      return (
        status?.build?.status === "running" ||
        status?.build?.status === "building_image" ||
        status?.build?.status === "pushing_image" ||
        status?.build?.status === "deploying" ||
        status?.build?.status === "pending" ||
        projectStatus === "building" ||
        projectStatus === "pending" ||
        projectStatus === "pending_initial_build"
      );
    });

    if (!needsPolling) {
      return;
    }

    const pollInterval = setInterval(() => {
      fetchProjectStatuses();
    }, 5000); // Poll every 5 seconds

    return () => {
      clearInterval(pollInterval);
    };
  }, [accessToken, authLoading, isAuthenticated, isLoading, projects, projectStatuses]);

  // Get actual project status based on build and deployment (sync with BuildStatusBadge)
  const getProjectStatusInfo = (project: any): { status: StatusType; label: string; className: string } => {
    const status = projectStatuses[project.id];
    
    // Priority 1: Check build status (sync with BuildStatusBadge)
    if (status?.build) {
      const buildStatus = status.build.status;
      switch (buildStatus) {
        case "pending":
          return {
            status: "pending",
            label: "Pending",
            className: "bg-yellow-500/10 text-yellow-500",
          };
        case "running":
          return {
            status: "building",
            label: "Running",
            className: "bg-blue-500/10 text-blue-500",
          };
        case "building_image":
          return {
            status: "building",
            label: "Building Image",
            className: "bg-blue-500/10 text-blue-500",
          };
        case "pushing_image":
          return {
            status: "building",
            label: "Pushing Image",
            className: "bg-blue-500/10 text-blue-500",
          };
        case "deploying":
          return {
            status: "building",
            label: "Deploying",
            className: "bg-purple-500/10 text-purple-500",
          };
        case "success":
          // If build succeeded, check deployment
          if (status.deployment?.status === "running") {
            return {
              status: "running",
              label: "Running",
              className: "bg-accent-emerald/10 text-accent-emerald",
            };
          }
          if (status.deployment?.status === "deploying") {
            return {
              status: "building",
              label: "Deploying",
              className: "bg-purple-500/10 text-purple-500",
            };
          }
          // Build succeeded but not deployed
          return {
            status: "stopped",
            label: "Stopped",
            className: "bg-surface-800 text-surface-400",
          };
        case "failed":
        case "deploy_failed":
          return {
            status: "error",
            label: "Failed",
            className: "bg-accent-rose/10 text-accent-rose",
          };
      }
    }

    // Priority 2: Check deployment status if no builds
    if (status?.deployment) {
      if (status.deployment.status === "running") {
        return {
          status: "running",
          label: "Running",
          className: "bg-accent-emerald/10 text-accent-emerald",
        };
      }
      if (status.deployment.status === "deploying") {
        return {
          status: "building",
          label: "Deploying",
          className: "bg-purple-500/10 text-purple-500",
        };
      }
      if (status.deployment.status === "stopped") {
        return {
          status: "stopped",
          label: "Stopped",
          className: "bg-surface-800 text-surface-400",
        };
      }
    }

    // Fallback: Use project status from API
    const statusMap: Record<string, StatusType> = {
      running: "running",
      stopped: "stopped",
      building: "building",
      error: "error",
      failed: "error",
      pending: "pending",
      pending_initial_build: "pending",
    };
    const mappedStatus = statusMap[project.status] || "pending";
    const statusConfig: Record<StatusType, { label: string; className: string }> = {
      running: {
        label: "Running",
        className: "bg-accent-emerald/10 text-accent-emerald",
      },
      stopped: {
        label: "Stopped",
        className: "bg-surface-800 text-surface-400",
      },
      building: {
        label: "Building",
        className: "bg-accent-amber/10 text-accent-amber",
      },
      error: {
        label: "Error",
        className: "bg-accent-rose/10 text-accent-rose",
      },
      pending: {
        label: "Pending",
        className: "bg-purple-500/10 text-purple-500",
      },
    };
    return {
      status: mappedStatus,
      ...statusConfig[mappedStatus],
    };
  };

  // Calculate stats based on actual project statuses
  const stats = {
    total: projects.length,
    running: projects.filter((p) => {
      const info = getProjectStatusInfo(p);
      return info.status === "running";
    }).length,
    building: projects.filter((p) => {
      const info = getProjectStatusInfo(p);
      return info.status === "building";
    }).length,
    errors: projects.filter((p) => {
      const info = getProjectStatusInfo(p);
      return info.status === "error";
    }).length,
  };

  return (
    <div className="relative min-h-screen bg-background">
      {/* Dot Grid Background */}
      <DotGrid dotColor="#27272a" spacing={28} fadeEdges />

      <Navbar />

      <div className="flex">
        <Sidebar />

        {/* Main Content */}
        <main className="flex-1 lg:ml-64 pt-16">
          <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
            {/* Header */}
            <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h1 className="text-2xl font-bold text-foreground sm:text-3xl">Dashboard</h1>
                <p className="mt-1 text-surface-400">
                  Manage and monitor your deployments
                </p>
              </div>
              <Link
                href="/projects/new"
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-white transition-all duration-200 hover:bg-primary-600 hover:shadow-lg hover:shadow-primary/20"
              >
                <Plus className="h-4 w-4" />
                New Project
              </Link>
            </div>

            {/* Stats Grid */}
            <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4 backdrop-blur-sm">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                    <Server className="h-5 w-5 text-primary" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold text-foreground">{stats.total}</p>
                    <p className="text-xs text-surface-400">Total Projects</p>
                  </div>
                </div>
              </div>

              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4 backdrop-blur-sm">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-accent-emerald/10">
                    <Activity className="h-5 w-5 text-accent-emerald" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold text-foreground">{stats.running}</p>
                    <p className="text-xs text-surface-400">Running</p>
                  </div>
                </div>
              </div>

              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4 backdrop-blur-sm">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-accent-amber/10">
                    <RefreshCw className="h-5 w-5 text-accent-amber" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold text-foreground">{stats.building}</p>
                    <p className="text-xs text-surface-400">Building</p>
                  </div>
                </div>
              </div>

              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-4 backdrop-blur-sm">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-accent-rose/10">
                    <AlertTriangle className="h-5 w-5 text-accent-rose" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold text-foreground">{stats.errors}</p>
                    <p className="text-xs text-surface-400">Errors</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Projects Section */}
            <div>
              <h2 className="mb-4 text-lg font-semibold text-foreground">Your Projects</h2>

              {isLoading ? (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {[1, 2, 3].map((i) => (
                    <div
                      key={i}
                      className="h-48 animate-pulse rounded-xl border border-surface-800 bg-surface-900/50"
                    />
                  ))}
                </div>
              ) : error ? (
                <div className="rounded-xl border border-accent-rose/30 bg-accent-rose/10 p-6 text-center">
                  <AlertTriangle className="mx-auto mb-3 h-8 w-8 text-accent-rose" />
                  <p className="text-foreground">{error}</p>
                </div>
              ) : projects.length === 0 ? (
                <div className="rounded-xl border border-dashed border-surface-700 bg-surface-900/30 p-12 text-center">
                  <Server className="mx-auto mb-4 h-12 w-12 text-surface-600" />
                  <h3 className="mb-2 text-lg font-medium text-foreground">No projects yet</h3>
                  <p className="mb-6 text-surface-400">
                    Get started by creating your first project
                  </p>
                  <Link
                    href="/projects/new"
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600"
                  >
                    <Plus className="h-4 w-4" />
                    Create Project
                  </Link>
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {projects.map((project) => {
                    const statusInfo = getProjectStatusInfo(project);
                    const isAnimated =
                      statusInfo.status === "building" &&
                      (projectStatuses[project.id]?.build?.status === "running" ||
                        projectStatuses[project.id]?.build?.status === "building_image" ||
                        projectStatuses[project.id]?.build?.status === "pushing_image" ||
                        projectStatuses[project.id]?.build?.status === "deploying");
                    return (
                      <Link key={project.id} href={`/projects/${project.id}`}>
                        <AnimatedCard status={statusInfo.status} className="h-full cursor-pointer">
                          <div className="flex h-full flex-col">
                            <div className="mb-4 flex items-start justify-between">
                              <h3 className="text-lg font-semibold text-foreground">
                                {project.name}
                              </h3>
                              <span
                                className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${statusInfo.className}`}
                              >
                                {isAnimated && (
                                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                                )}
                                {statusInfo.label}
                              </span>
                            </div>

                            <div className="flex-1 space-y-3">
                              <div className="flex items-center gap-2 text-sm text-surface-400">
                                <GitBranch className="h-4 w-4" />
                                <span className="truncate">{project.branch || "main"}</span>
                              </div>
                              <div className="flex items-center gap-2 text-sm text-surface-400">
                                <Clock className="h-4 w-4" />
                                <span>
                                  {new Date(project.updated_at).toLocaleDateString()}
                                </span>
                              </div>
                            </div>

                            <div className="mt-4 flex items-center justify-between border-t border-surface-800 pt-4">
                              <span className="rounded-md bg-surface-800 px-2 py-1 text-xs font-medium text-surface-300">
                                {project.preset}
                              </span>
                              <ExternalLink className="h-4 w-4 text-surface-500 transition-colors group-hover:text-foreground" />
                            </div>
                          </div>
                        </AnimatedCard>
                      </Link>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

