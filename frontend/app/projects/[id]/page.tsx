"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import { Project } from "@/lib/store/projectStore";
import { projectApi } from "@/lib/api/projects";
import { buildsApi, Build } from "@/lib/api/builds";
import { deploymentsApi, Deployment } from "@/lib/api/deployments";
import { BuildCard } from "@/components/projects/BuildCard";
import { BuildLogs } from "@/components/projects/BuildLogs";
import {
  ArrowLeft,
  GitBranch,
  Clock,
  ExternalLink,
  Play,
  Square,
  RefreshCw,
  Trash2,
  Settings,
  Terminal,
  Key,
  AlertTriangle,
  CheckCircle,
  Loader2,
  XCircle,
} from "lucide-react";

type TabType = "overview" | "builds" | "settings" | "secrets";

const statusConfig = {
  running: {
    label: "Running",
    color: "text-accent-emerald",
    bg: "bg-accent-emerald/10",
    icon: CheckCircle,
  },
  stopped: {
    label: "Stopped",
    color: "text-surface-400",
    bg: "bg-surface-800",
    icon: Square,
  },
  building: {
    label: "Building",
    color: "text-accent-amber",
    bg: "bg-accent-amber/10",
    icon: RefreshCw,
  },
  error: {
    label: "Error",
    color: "text-accent-rose",
    bg: "bg-accent-rose/10",
    icon: XCircle,
  },
  pending_initial_build: {
    label: "Pending",
    color: "text-purple-500",
    bg: "bg-purple-500/10",
    icon: Clock,
  },
};

export default function ProjectDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { accessToken, isLoading: authLoading } = useAuthStore();
  const projectId = params.id as string;

  const [project, setProject] = useState<Project | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabType>("overview");
  const [isDeleting, setIsDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Builds state
  const [builds, setBuilds] = useState<Build[]>([]);
  const [buildsLoading, setBuildsLoading] = useState(false);
  const [expandedBuildId, setExpandedBuildId] = useState<string | null>(null);
  const [triggeringBuild, setTriggeringBuild] = useState(false);
  const [logsClearedTimestamp, setLogsClearedTimestamp] = useState<number>(0);

  // Deployment state
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [deploymentLoading, setDeploymentLoading] = useState(false);
  const [isDeploying, setIsDeploying] = useState(false);
  const [isStopping, setIsStopping] = useState(false);
  const [isRebuilding, setIsRebuilding] = useState(false);

  // Build & Deploy workflow state
  const [buildAndDeployStep, setBuildAndDeployStep] = useState<
    "idle" | "building" | "deploying" | "success" | "failed"
  >("idle");
  const [currentBuildId, setCurrentBuildId] = useState<string | null>(null);
  const deployTriggeredRef = useRef<string | null>(null);

  useEffect(() => {
    const fetchProject = async () => {
      // Wait for auth rehydration to complete
      if (authLoading) {
        return;
      }

      if (!accessToken) {
        setError("Not authenticated");
        setIsLoading(false);
        return;
      }

      try {
        setIsLoading(true);
        setError(null);
        const data = await projectApi.getProject(accessToken, projectId);
        setProject(data);
      } catch (err: any) {
        console.error("Failed to fetch project:", err);
        setError(err.message || "Failed to fetch project");
      } finally {
        setIsLoading(false);
      }
    };

    fetchProject();
  }, [projectId, accessToken, authLoading]);

  // Poll project status if project is building or pending
  useEffect(() => {
    if (!accessToken || authLoading || !project || isLoading) {
      return;
    }

    // Check if project is in a state that needs polling
    // Note: project.status from API might be "pending_initial_build" even though type says "pending"
    const projectStatus = (project as any).status as string;
    const needsPolling =
      projectStatus === "building" ||
      projectStatus === "pending" ||
      projectStatus === "pending_initial_build";

    if (!needsPolling) {
      return;
    }

    const pollProject = async () => {
      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) {
          return;
        }

        const data = await projectApi.getProject(currentToken, projectId);
        setProject(data);
      } catch (err: any) {
        console.error("Failed to poll project:", err);
      }
    };

    // Start polling
    const pollInterval = setInterval(pollProject, 5000); // Poll every 5 seconds

    return () => {
      clearInterval(pollInterval);
    };
  }, [projectId, accessToken, authLoading, isLoading, project]);

  // Fetch deployment status
  useEffect(() => {
    const fetchDeployment = async () => {
      if (!accessToken || !project) return;

      try {
        setDeploymentLoading(true);
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;
        
        const data = await deploymentsApi.getDeploymentStatus(
          currentToken,
          projectId
        );
        setDeployment(data);
      } catch (err: any) {
        // Deployment might not exist yet, that's okay
        // Backend now returns 200 with null deployment instead of 404
        console.error("Failed to fetch deployment:", err);
        setDeployment(null);
      } finally {
        setDeploymentLoading(false);
      }
    };

    if (project) {
      fetchDeployment();
    }
  }, [projectId, accessToken, project]);

  // Poll deployment status when deploying or when build status is deploying
  useEffect(() => {
    // Check if we should poll (either buildAndDeployStep is deploying, or latest build is deploying)
    const shouldPoll = 
      buildAndDeployStep === "deploying" || 
      (builds.length > 0 && builds[0].status === "deploying");

    if (!shouldPoll || !accessToken || !project) {
      return;
    }

    const pollDeployment = setInterval(async () => {
      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) {
          clearInterval(pollDeployment);
          return;
        }
        
        const data = await deploymentsApi.getDeploymentStatus(
          currentToken,
          projectId
        );
        setDeployment(data);
        
        // If deployment is running, stop polling
        if (data?.status === "running") {
          clearInterval(pollDeployment);
          setBuildAndDeployStep("success");
          // Refresh project status
          const updatedProject = await projectApi.getProject(currentToken, projectId);
          setProject(updatedProject);
          // Reset to idle after 2 seconds
          setTimeout(() => {
            setBuildAndDeployStep("idle");
          }, 2000);
        }
      } catch (err: any) {
        console.error("Failed to poll deployment:", err);
      }
    }, 2000); // Poll every 2 seconds

    return () => clearInterval(pollDeployment);
  }, [projectId, accessToken, project, buildAndDeployStep, builds]);

  // Fetch latest build for overview (always fetch first build)
  useEffect(() => {
    const fetchLatestBuild = async () => {
      if (!accessToken || !project) return;

      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;
        
        const response = await buildsApi.listBuilds(currentToken, projectId, 1, 1);
        if (response.builds && response.builds.length > 0) {
          setBuilds((prev) => {
            // Only update if we don't have builds or if this is newer
            if (prev.length === 0 || prev[0].id !== response.builds[0].id) {
              return response.builds;
            }
            // Update status if build is still running
            if (prev[0].id === response.builds[0].id) {
              const updated = [...prev];
              updated[0] = response.builds[0];
              return updated;
            }
            return prev;
          });
        }
      } catch (err: any) {
        console.error("Failed to fetch latest build:", err);
      }
    };

    if (project) {
      fetchLatestBuild();
      // Poll latest build status if it's still running
      const pollInterval = setInterval(() => {
        fetchLatestBuild();
      }, 5000); // Poll every 5 seconds

      return () => clearInterval(pollInterval);
    }
  }, [projectId, accessToken, project]);

  // Fetch builds when builds tab is active
  useEffect(() => {
    const fetchBuilds = async () => {
      if (!accessToken || activeTab !== "builds") return;

      try {
        setBuildsLoading(true);
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;
        
        const response = await buildsApi.listBuilds(currentToken, projectId);
        setBuilds(response.builds || []);
      } catch (err: any) {
        console.error("Failed to fetch builds:", err);
      } finally {
        setBuildsLoading(false);
      }
    };

    if (activeTab === "builds") {
      fetchBuilds();
    }
  }, [projectId, accessToken, activeTab]);

  const handleDelete = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    setIsDeleting(true);
    try {
      await projectApi.deleteProject(accessToken, projectId);
      router.push("/dashboard");
    } catch (err: any) {
      console.error("Failed to delete project:", err);
      setError(err.message || "Failed to delete project");
    } finally {
      setIsDeleting(false);
      setShowDeleteConfirm(false);
    }
  };

  const handleDeploy = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    setIsDeploying(true);
    setError(null);
    try {
      const deployment = await deploymentsApi.deploy(accessToken, projectId);
      setDeployment(deployment);
      // Refresh project status
      const updatedProject = await projectApi.getProject(accessToken, projectId);
      setProject(updatedProject);
    } catch (err: any) {
      console.error("Failed to deploy:", err);
      setError(err.message || "Failed to deploy");
    } finally {
      setIsDeploying(false);
    }
  };

  const handleBuildAndDeploy = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    deployTriggeredRef.current = null;
    setBuildAndDeployStep("building");
    setError(null);
    
    // Auto-switch to builds tab to show progress
    setActiveTab("builds");

    try {
      // Step 1: Trigger build with project info
      if (!project) {
        setError("Project information not available");
        setBuildAndDeployStep("failed");
        return;
      }
      const build = await buildsApi.triggerBuild(
        accessToken,
        projectId,
        undefined, // commit_sha
        project.branch,
        project.repo_url
      );
      setCurrentBuildId(build.id);
      setBuilds((prev) => [build, ...prev]);
      setExpandedBuildId(build.id); // Auto-expand build logs

      // Step 2: Poll build status (still keep polling for updates)
      const pollBuildStatus = (buildId: string) => {
        const maxAttempts = 120; // 10 minutes max (poll every 5s)
        let attempts = 0;

        const poll = setInterval(async () => {
          attempts++;
          try {
            // Get fresh accessToken from store on each poll
            const currentToken = useAuthStore.getState().accessToken;
            if (!currentToken) {
              clearInterval(poll);
              setError("Authentication expired. Please refresh the page.");
              setBuildAndDeployStep("failed");
              return;
            }
            const buildDetails = await buildsApi.getBuild(currentToken, buildId);
            setBuilds((prev) =>
              prev.map((b) => (b.id === buildId ? buildDetails : b))
            );

            if (
              buildDetails.status === "success" ||
              buildDetails.status === "failed" ||
              buildDetails.status === "deploy_failed"
            ) {
              clearInterval(poll);
              if (buildDetails.status === "failed" || buildDetails.status === "deploy_failed") {
                setError("Build failed. Please check build logs for details.");
                setBuildAndDeployStep("failed");
              } else if (buildDetails.status === "success") {
                // Step 3: Deploy after successful build
                if (deployTriggeredRef.current === buildId) {
                  return;
                }
                deployTriggeredRef.current = buildId;
                setBuildAndDeployStep("deploying");
                try {
                  const currentToken = useAuthStore.getState().accessToken;
                  if (!currentToken) {
                    setError("Authentication expired. Please refresh the page.");
                    setBuildAndDeployStep("failed");
                    return;
                  }
                  const deployment = await deploymentsApi.deploy(currentToken, projectId);
                  setDeployment(deployment);

                  // Refresh project status
                  const updatedProject = await projectApi.getProject(currentToken, projectId);
                  setProject(updatedProject);

                  setBuildAndDeployStep("success");
                  setCurrentBuildId(null);

                  // Reset to idle after 2 seconds
                  setTimeout(() => {
                    setBuildAndDeployStep("idle");
                  }, 2000);
                } catch (deployErr: any) {
                  console.error("Failed to deploy:", deployErr);
                  setError(deployErr.message || "Failed to deploy");
                  setBuildAndDeployStep("failed");
                }
              }
            } else if (attempts >= maxAttempts) {
              clearInterval(poll);
              setError("Build timeout. Please check build status manually.");
              setBuildAndDeployStep("failed");
            }
          } catch (err) {
            console.error("Failed to poll build status:", err);
            const status = (err as any)?.status;
            // Nếu build đã bị xóa / 404, dừng poll và cập nhật UI
            if (status === 404) {
              clearInterval(poll);
              setError("Build not found or has been deleted.");
              setBuildAndDeployStep("failed");
              setCurrentBuildId(null);
              setExpandedBuildId(null);
              setBuilds((prev) => prev.filter((b) => b.id !== buildId));
            }
          }
        }, 5000); // Poll every 5 seconds

        // Return poll id to allow manual clear if needed
        return poll;
      };

      pollBuildStatus(build.id);
    } catch (err: any) {
      console.error("Failed to trigger build:", err);
      setError(err.message || "Failed to trigger build");
      setBuildAndDeployStep("failed");
    }
  };

  // Fallback: if build becomes success but deploy not triggered (e.g., poll hiccup), trigger deploy once
  useEffect(() => {
    if (!project || !accessToken) return;
    if (buildAndDeployStep !== "building" && buildAndDeployStep !== "deploying") return;

    const targetBuild = currentBuildId
      ? builds.find((b) => b.id === currentBuildId)
      : builds[0];

    if (!targetBuild || targetBuild.status !== "success") return;
    if (deployTriggeredRef.current === targetBuild.id) return;

    deployTriggeredRef.current = targetBuild.id;
    (async () => {
      setBuildAndDeployStep("deploying");
      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) {
          setError("Authentication expired. Please refresh the page.");
          setBuildAndDeployStep("failed");
          return;
        }
        const deployment = await deploymentsApi.deploy(currentToken, projectId);
        setDeployment(deployment);
        const updatedProject = await projectApi.getProject(currentToken, projectId);
        setProject(updatedProject);
        setBuildAndDeployStep("success");
        setCurrentBuildId(null);
        setTimeout(() => setBuildAndDeployStep("idle"), 2000);
      } catch (err: any) {
        console.error("Fallback deploy failed:", err);
        setError(err?.message || "Failed to deploy");
        setBuildAndDeployStep("failed");
      }
    })();
  }, [builds, currentBuildId, project, accessToken, buildAndDeployStep, projectId]);

  const handleStop = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    if (!confirm("Are you sure you want to stop this deployment?")) {
      return;
    }

    setIsStopping(true);
    setError(null);
    try {
      await deploymentsApi.stopDeployment(accessToken, projectId);
      setDeployment(null);
      // Refresh project status
      const updatedProject = await projectApi.getProject(accessToken, projectId);
      setProject(updatedProject);
    } catch (err: any) {
      console.error("Failed to stop deployment:", err);
      setError(err.message || "Failed to stop deployment");
    } finally {
      setIsStopping(false);
    }
  };

  const handleRebuild = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    setIsRebuilding(true);
    setError(null);
    try {
      // Trigger build
      const build = await buildsApi.triggerBuild(accessToken, projectId);
      setBuilds((prev) => [build, ...prev]);

      // Wait for build to complete (polling)
      const pollBuildStatus = async (buildId: string) => {
        const maxAttempts = 60; // 5 minutes max
        let attempts = 0;

        const poll = setInterval(async () => {
          attempts++;
          try {
            // Get fresh accessToken from store on each poll
            const currentToken = useAuthStore.getState().accessToken;
            if (!currentToken) {
              clearInterval(poll);
              setIsRebuilding(false);
              setError("Authentication expired. Please refresh the page.");
              return;
            }
            const buildDetails = await buildsApi.getBuild(currentToken, buildId);
            setBuilds((prev) =>
              prev.map((b) => (b.id === buildId ? buildDetails : b))
            );

            if (
              buildDetails.status === "success" ||
              buildDetails.status === "failed" ||
              buildDetails.status === "deploy_failed"
            ) {
              clearInterval(poll);
              // If successful, auto-deploy
              if (buildDetails.status === "success") {
                await handleDeploy();
              }
              setIsRebuilding(false);
            } else if (attempts >= maxAttempts) {
              clearInterval(poll);
              setIsRebuilding(false);
              setError("Build timeout. Please check build status manually.");
            }
          } catch (err) {
            console.error("Failed to poll build status:", err);
          }
        }, 5000); // Poll every 5 seconds
      };

      await pollBuildStatus(build.id);
    } catch (err: any) {
      console.error("Failed to rebuild:", err);
      setError(err.message || "Failed to rebuild");
      setIsRebuilding(false);
    }
  };

  const handleClearBuildLogs = async () => {
    if (!accessToken) {
      setError("Not authenticated");
      return;
    }

    if (!confirm("Are you sure you want to clear all build history? This will delete all builds and their logs. This action cannot be undone.")) {
      return;
    }

    setTriggeringBuild(true);
    setError(null);
    try {
      // Get current builds to clear their cache
      const currentBuildIds = builds.map(b => b.id);
      
      await buildsApi.clearBuildLogs(accessToken, projectId);
      
      // Clear cache for all builds in this project
      const { buildLogsCache } = await import("@/lib/cache/buildLogsCache");
      // Clear all cache to ensure no stale logs remain
      buildLogsCache.clearAll();
      
      // Clear expanded build to force reload if user reopens
      setExpandedBuildId(null);
      
      // Set timestamp to force remount of BuildLogs components
      setLogsClearedTimestamp(Date.now());
      
      // Refresh builds list
      const response = await buildsApi.listBuilds(accessToken, projectId);
      setBuilds(response.builds || []);
      
      // Clear any error state on success
      setError(null);
    } catch (err: any) {
      console.error("Failed to clear build logs:", err);
      setError(err.message || "Failed to clear build logs");
    } finally {
      setTriggeringBuild(false);
    }
  };

  const getStatusConfig = (status: string) => {
    // Map backend status to frontend status
    const statusMap: Record<string, keyof typeof statusConfig> = {
      running: "running",
      stopped: "stopped",
      building: "building",
      error: "error",
      failed: "error",
      pending: "pending_initial_build",
      pending_initial_build: "pending_initial_build",
    };
    const mappedStatus = statusMap[status] || "pending_initial_build";
    return statusConfig[mappedStatus];
  };

  const tabs = [
    { id: "overview", label: "Overview", icon: Terminal },
    { id: "builds", label: "Builds", icon: RefreshCw },
    { id: "settings", label: "Settings", icon: Settings },
    { id: "secrets", label: "Secrets", icon: Key },
  ] as const;

  // Show loading while auth is rehydrating or project is loading
  if (authLoading || isLoading) {
    return (
      <div className="relative min-h-screen bg-background">
        <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
        <Navbar />
        <div className="flex">
          <Sidebar />
          <main className="flex-1 lg:ml-64 pt-16">
            <div className="flex min-h-[60vh] items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          </main>
        </div>
      </div>
    );
  }

  // Determine actual project status based on build status (sync with BuildStatusBadge)
  const getActualProjectStatus = (): { status: string; label: string; color: string; bg: string; icon: any } => {
    if (!project) {
      return {
        status: "pending_initial_build",
        label: "Pending",
        color: "text-purple-500",
        bg: "bg-purple-500/10",
        icon: Clock,
      };
    }
    
    // Priority 1: Check latest build status (sync with BuildStatusBadge)
    if (builds.length > 0) {
      const latestBuild = builds[0];
      const buildStatus = latestBuild.status;
      
      // Map build status to project status badge (same as BuildStatusBadge)
      switch (buildStatus) {
        case "pending":
          return {
            status: "pending",
            label: "Pending",
            color: "text-yellow-500",
            bg: "bg-yellow-500/10",
            icon: Clock,
          };
        case "running":
          return {
            status: "running",
            label: "Running",
            color: "text-blue-500",
            bg: "bg-blue-500/10",
            icon: Loader2,
          };
        case "building_image":
          return {
            status: "building_image",
            label: "Building Image",
            color: "text-blue-500",
            bg: "bg-blue-500/10",
            icon: Loader2,
          };
        case "pushing_image":
          return {
            status: "pushing_image",
            label: "Pushing Image",
            color: "text-blue-500",
            bg: "bg-blue-500/10",
            icon: Loader2,
          };
        case "deploying":
          return {
            status: "deploying",
            label: "Deploying",
            color: "text-purple-500",
            bg: "bg-purple-500/10",
            icon: Loader2,
          };
        case "success":
          // If build succeeded, check deployment status
          if (deployment?.status === "running") {
            return {
              status: "running",
              label: "Running",
              color: "text-accent-emerald",
              bg: "bg-accent-emerald/10",
              icon: CheckCircle,
            };
          }
          if (deployment?.status === "deploying" || buildAndDeployStep === "deploying") {
            return {
              status: "deploying",
              label: "Deploying",
              color: "text-purple-500",
              bg: "bg-purple-500/10",
              icon: Loader2,
            };
          }
          // Build succeeded but not deployed
          return {
            status: "stopped",
            label: "Stopped",
            color: "text-surface-400",
            bg: "bg-surface-800",
            icon: Square,
          };
        case "failed":
        case "deploy_failed":
          return {
            status: "error",
            label: "Failed",
            color: "text-accent-rose",
            bg: "bg-accent-rose/10",
            icon: XCircle,
          };
      }
    }
    
    // Priority 2: Check deployment status if no builds
    if (deployment) {
      if (deployment.status === "running") {
        return {
          status: "running",
          label: "Running",
          color: "text-accent-emerald",
          bg: "bg-accent-emerald/10",
          icon: CheckCircle,
        };
      }
      if (deployment.status === "deploying" || buildAndDeployStep === "deploying") {
        return {
          status: "deploying",
          label: "Deploying",
          color: "text-purple-500",
          bg: "bg-purple-500/10",
          icon: Loader2,
        };
      }
      if (deployment.status === "stopped") {
        return {
          status: "stopped",
          label: "Stopped",
          color: "text-surface-400",
          bg: "bg-surface-800",
          icon: Square,
        };
      }
    }
    
    // Priority 3: Check buildAndDeployStep
    if (buildAndDeployStep === "building") {
      return {
        status: "building",
        label: "Building",
        color: "text-accent-amber",
        bg: "bg-accent-amber/10",
        icon: RefreshCw,
      };
    }
    if (buildAndDeployStep === "deploying") {
      return {
        status: "deploying",
        label: "Deploying",
        color: "text-purple-500",
        bg: "bg-purple-500/10",
        icon: Loader2,
      };
    }
    
    // Fallback: Use project status from API
    const fallbackStatus = getStatusConfig(project.status || "pending_initial_build");
    return {
      status: project.status || "pending_initial_build",
      label: fallbackStatus.label,
      color: fallbackStatus.color,
      bg: fallbackStatus.bg,
      icon: fallbackStatus.icon,
    };
  };

  const statusInfo = getActualProjectStatus();
  const StatusIcon = statusInfo.icon;

  return (
    <div className="relative min-h-screen bg-background">
      <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
      <Navbar />

      <div className="flex">
        <Sidebar />

        <main className="flex-1 lg:ml-64 pt-16">
          <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
            {/* Error Notification Card */}
            {error && (
              <div className="mb-6 rounded-lg border border-accent-rose/20 bg-accent-rose/10 p-4">
                <div className="flex items-start gap-3">
                  <AlertTriangle className="h-5 w-5 shrink-0 text-accent-rose" />
                  <div className="flex-1">
                    <h3 className="text-sm font-semibold text-accent-rose mb-1">
                      {!project ? "Project not found" : "Error"}
                    </h3>
                    <p className="text-sm text-surface-300">{error}</p>
                  </div>
                  <button
                    onClick={() => setError(null)}
                    className="shrink-0 text-surface-400 hover:text-foreground transition-colors"
                  >
                    <XCircle className="h-4 w-4" />
                  </button>
                </div>
              </div>
            )}

            {/* Header */}
            <div className="mb-6">
              <Link
                href="/dashboard"
                className="mb-4 inline-flex items-center gap-2 text-sm text-surface-400 transition-colors hover:text-foreground"
              >
                <ArrowLeft className="h-4 w-4" />
                Back to Dashboard
              </Link>

              {project && (
                <>
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold text-foreground sm:text-3xl">
                          {project.name}
                        </h1>
                        <span
                          className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${statusInfo.bg} ${statusInfo.color}`}
                        >
                          {(statusInfo.status === "running" ||
                            statusInfo.status === "building_image" ||
                            statusInfo.status === "pushing_image" ||
                            statusInfo.status === "deploying") ? (
                            <StatusIcon className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <StatusIcon className="h-3.5 w-3.5" />
                          )}
                          {statusInfo.label}
                        </span>
                      </div>
                      <div className="mt-2 flex items-center gap-4 text-sm text-surface-400">
                        <span className="flex items-center gap-1.5">
                          <GitBranch className="h-4 w-4" />
                          {project.branch}
                        </span>
                        <span className="flex items-center gap-1.5">
                          <Clock className="h-4 w-4" />
                          Updated {new Date(project.updated_at).toLocaleDateString()}
                        </span>
                      </div>
                    </div>

                    {/* Build & Deploy Progress Indicator */}
                    {buildAndDeployStep !== "idle" && (
                      <div className="mb-4 rounded-lg border border-surface-800 bg-surface-900/50 p-4">
                        <div className="flex items-center gap-3">
                          <div className="flex items-center gap-2">
                            {buildAndDeployStep === "building" && (
                              <>
                                <Loader2 className="h-4 w-4 animate-spin text-accent-amber" />
                                <span className="text-sm font-medium text-foreground">
                                  Building...
                                </span>
                              </>
                            )}
                            {buildAndDeployStep === "deploying" && (
                              <>
                                <Loader2 className="h-4 w-4 animate-spin text-accent-emerald" />
                                <span className="text-sm font-medium text-foreground">
                                  Deploying...
                                </span>
                              </>
                            )}
                            {buildAndDeployStep === "success" && (
                              <>
                                <CheckCircle className="h-4 w-4 text-accent-emerald" />
                                <span className="text-sm font-medium text-accent-emerald">
                                  Successfully deployed!
                                </span>
                              </>
                            )}
                            {buildAndDeployStep === "failed" && (
                              <>
                                <AlertTriangle className="h-4 w-4 text-accent-rose" />
                                <span className="text-sm font-medium text-accent-rose">
                                  Build & Deploy failed
                                </span>
                              </>
                            )}
                          </div>
                          {currentBuildId && (
                            <button
                              onClick={() => {
                                setActiveTab("builds");
                                setExpandedBuildId(currentBuildId);
                              }}
                              className="ml-auto text-xs text-surface-400 hover:text-foreground"
                            >
                              View logs →
                            </button>
                          )}
                        </div>
                      </div>
                    )}

                    <div className="flex items-center gap-2">
                      {deployment?.status === "running" ? (
                        <button
                          onClick={handleStop}
                          disabled={isStopping || buildAndDeployStep !== "idle"}
                          className="inline-flex items-center gap-2 rounded-lg border border-surface-700 px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surface-800 disabled:opacity-50"
                        >
                          {isStopping ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Square className="h-4 w-4" />
                          )}
                          Stop
                        </button>
                      ) : (
                        <button
                          onClick={handleBuildAndDeploy}
                          disabled={
                            buildAndDeployStep !== "idle" ||
                            deploymentLoading ||
                            isDeploying
                          }
                          className="inline-flex items-center gap-2 rounded-lg bg-accent-emerald px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-emerald/90 disabled:opacity-50"
                        >
                          {buildAndDeployStep === "building" ? (
                            <>
                              <Loader2 className="h-4 w-4 animate-spin" />
                              Building...
                            </>
                          ) : buildAndDeployStep === "deploying" ? (
                            <>
                              <Loader2 className="h-4 w-4 animate-spin" />
                              Deploying...
                            </>
                          ) : buildAndDeployStep === "success" ? (
                            <>
                              <CheckCircle className="h-4 w-4" />
                              Deployed
                            </>
                          ) : (
                            <>
                              <Play className="h-4 w-4" />
                              Build & Deploy
                            </>
                          )}
                        </button>
                      )}
                      <button
                        onClick={handleRebuild}
                        disabled={isRebuilding}
                        className="inline-flex items-center gap-2 rounded-lg border border-surface-700 px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surface-800 disabled:opacity-50"
                      >
                        {isRebuilding ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <RefreshCw className="h-4 w-4" />
                        )}
                        Rebuild
                      </button>
                      <a
                        href={project.repo_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="rounded-lg border border-surface-700 p-2 text-surface-400 transition-colors hover:bg-surface-800 hover:text-foreground"
                      >
                        <ExternalLink className="h-5 w-5" />
                      </a>
                    </div>
                  </div>
                </>
              )}
            </div>

            {/* Tabs - Only show if project exists */}
            {project && (
              <>
                <div className="mb-6 border-b border-surface-800">
                  <div className="flex gap-1">
                    {tabs.map((tab) => {
                      const TabIcon = tab.icon;
                      return (
                        <button
                          key={tab.id}
                          onClick={() => setActiveTab(tab.id)}
                          className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-medium transition-colors ${
                            activeTab === tab.id
                              ? "border-primary text-primary"
                              : "border-transparent text-surface-400 hover:text-foreground"
                          }`}
                        >
                          <TabIcon className="h-4 w-4" />
                          {tab.label}
                        </button>
                      );
                    })}
                  </div>
                </div>

                {/* Tab Content */}
                {activeTab === "overview" && (
              <div className="grid gap-6 lg:grid-cols-2">
                {/* Build Status Card */}
                <Card variant="elevated">
                  <div className="mb-4 flex items-center justify-between">
                    <h3 className="text-lg font-semibold text-foreground">
                      Build Status
                    </h3>
                    {builds.length > 0 && (
                      <button
                        onClick={() => setActiveTab("builds")}
                        className="text-xs text-surface-400 hover:text-foreground"
                      >
                        View all →
                      </button>
                    )}
                  </div>
                  {buildsLoading ? (
                    <div className="flex items-center justify-center py-8">
                      <Loader2 className="h-5 w-5 animate-spin text-primary" />
                    </div>
                  ) : builds.length > 0 ? (
                    <div className="space-y-4">
                      {(() => {
                        const latestBuild = builds[0];
                        const statusConfig = {
                          pending: {
                            label: "Pending",
                            color: "text-yellow-500",
                            bg: "bg-yellow-500/10",
                            icon: Clock,
                          },
                          running: {
                            label: "Running",
                            color: "text-blue-500",
                            bg: "bg-blue-500/10",
                            icon: Loader2,
                          },
                          building_image: {
                            label: "Building Image",
                            color: "text-blue-500",
                            bg: "bg-blue-500/10",
                            icon: Loader2,
                          },
                          pushing_image: {
                            label: "Pushing Image",
                            color: "text-blue-500",
                            bg: "bg-blue-500/10",
                            icon: Loader2,
                          },
                          deploying: {
                            label: "Deploying",
                            color: "text-purple-500",
                            bg: "bg-purple-500/10",
                            icon: Loader2,
                          },
                          success: {
                            label: "Success",
                            color: "text-accent-emerald",
                            bg: "bg-accent-emerald/10",
                            icon: CheckCircle,
                          },
                          failed: {
                            label: "Failed",
                            color: "text-accent-rose",
                            bg: "bg-accent-rose/10",
                            icon: XCircle,
                          },
                          deploy_failed: {
                            label: "Deploy Failed",
                            color: "text-accent-rose",
                            bg: "bg-accent-rose/10",
                            icon: XCircle,
                          },
                        };
                        const config =
                          statusConfig[
                            latestBuild.status as keyof typeof statusConfig
                          ] || {
                            label: latestBuild.status,
                            color: "text-surface-400",
                            bg: "bg-surface-800",
                            icon: Clock,
                          };
                        const StatusIcon = config.icon;
                        const isAnimated =
                          latestBuild.status === "running" ||
                          latestBuild.status === "building_image" ||
                          latestBuild.status === "pushing_image" ||
                          latestBuild.status === "deploying";

                        // Calculate duration
                        let duration = "—";
                        if (latestBuild.started_at && latestBuild.finished_at) {
                          const start = new Date(latestBuild.started_at).getTime();
                          const end = new Date(latestBuild.finished_at).getTime();
                          const seconds = Math.floor((end - start) / 1000);
                          duration = `${seconds}s`;
                        } else if (latestBuild.started_at) {
                          const start = new Date(latestBuild.started_at).getTime();
                          const now = Date.now();
                          const seconds = Math.floor((now - start) / 1000);
                          duration = `${seconds}s`;
                        }

                        // Format date
                        const buildDate = latestBuild.started_at
                          ? new Date(latestBuild.started_at).toLocaleString("vi-VN", {
                              day: "2-digit",
                              month: "2-digit",
                              year: "numeric",
                              hour: "2-digit",
                              minute: "2-digit",
                            })
                          : new Date(latestBuild.created_at).toLocaleString("vi-VN", {
                              day: "2-digit",
                              month: "2-digit",
                              year: "numeric",
                              hour: "2-digit",
                              minute: "2-digit",
                            });

                        return (
                          <>
                            <div>
                              <dt className="text-sm text-surface-400">Latest Build</dt>
                              <dd className="mt-1">
                                <span
                                  className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${
                                    config.bg
                                  } ${config.color}`}
                                >
                                  {isAnimated ? (
                                    <StatusIcon className="h-3 w-3 animate-spin" />
                                  ) : (
                                    <StatusIcon className="h-3 w-3" />
                                  )}
                                  {config.label}
                                </span>
                              </dd>
                            </div>
                            <div>
                              <dt className="text-sm text-surface-400">Build Time</dt>
                              <dd className="mt-1 text-sm text-foreground">
                                {buildDate}
                              </dd>
                            </div>
                            <div>
                              <dt className="text-sm text-surface-400">Duration</dt>
                              <dd className="mt-1 text-sm text-foreground">
                                {duration}
                              </dd>
                            </div>
                            {latestBuild.commit_sha && (
                              <div>
                                <dt className="text-sm text-surface-400">Commit</dt>
                                <dd className="mt-1 font-mono text-xs text-foreground">
                                  {latestBuild.commit_sha.substring(0, 8)}
                                </dd>
                              </div>
                            )}
                            <button
                              onClick={() => {
                                setActiveTab("builds");
                                setExpandedBuildId(latestBuild.id);
                              }}
                              className="w-full rounded-lg border border-surface-700 px-3 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surface-800"
                            >
                              View Build Logs
                            </button>
                          </>
                        );
                      })()}
                    </div>
                  ) : (
                    <div className="py-8 text-center text-sm text-surface-400">
                      No builds yet
                    </div>
                  )}
                </Card>

                {/* Deployment Status Card */}
                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">
                    Deployment Status
                  </h3>
                  {deploymentLoading ? (
                    <div className="flex items-center justify-center py-8">
                      <Loader2 className="h-5 w-5 animate-spin text-primary" />
                    </div>
                  ) : deployment ? (
                    <div className="space-y-4">
                      <div>
                        <dt className="text-sm text-surface-400">Status</dt>
                        <dd className="mt-1">
                          <span
                            className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${
                              deployment.status === "running"
                                ? "bg-accent-emerald/10 text-accent-emerald"
                                : deployment.status === "stopped"
                                ? "bg-surface-800 text-surface-400"
                                : deployment.status === "deploying" ||
                                  buildAndDeployStep === "deploying"
                                ? "bg-accent-amber/10 text-accent-amber"
                                : "bg-accent-rose/10 text-accent-rose"
                            }`}
                          >
                            {(deployment.status === "deploying" ||
                              buildAndDeployStep === "deploying") && (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            )}
                            {deployment.status === "running" && (
                              <CheckCircle className="h-3 w-3" />
                            )}
                            {deployment.status === "stopped" && (
                              <Square className="h-3 w-3" />
                            )}
                            {deployment.status === "deploying" ||
                            buildAndDeployStep === "deploying"
                              ? "Deploying..."
                              : deployment.status === "running"
                              ? "Running"
                              : deployment.status === "stopped"
                              ? "Stopped"
                              : deployment.status || "Unknown"}
                          </span>
                        </dd>
                      </div>
                      {deployment.public_url && (
                        <div>
                          <dt className="text-sm text-surface-400">Public URL</dt>
                          <dd className="mt-1">
                            <a
                              href={deployment.public_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-accent-emerald hover:underline"
                            >
                              {deployment.public_url}
                              <ExternalLink className="ml-1 inline h-3 w-3" />
                            </a>
                          </dd>
                        </div>
                      )}
                      {deployment.container_id && (
                        <div>
                          <dt className="text-sm text-surface-400">Container ID</dt>
                          <dd className="mt-1 font-mono text-xs text-foreground">
                            {deployment.container_id.substring(0, 12)}
                          </dd>
                        </div>
                      )}
                    </div>
                  ) : (
                    <div className="py-8 text-center text-sm text-surface-400">
                      No deployment yet
                    </div>
                  )}
                </Card>

                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">
                    Project Information
                  </h3>
                  <dl className="space-y-4">
                    <div>
                      <dt className="text-sm text-surface-400">Repository</dt>
                      <dd className="mt-1 font-mono text-sm text-foreground">
                        {project.repo_url}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-sm text-surface-400">Branch</dt>
                      <dd className="mt-1 text-foreground">{project.branch}</dd>
                    </div>
                    <div>
                      <dt className="text-sm text-surface-400">Preset</dt>
                      <dd className="mt-1 text-foreground capitalize">{project.preset}</dd>
                    </div>
                    <div>
                      <dt className="text-sm text-surface-400">Port</dt>
                      <dd className="mt-1 text-foreground">{project.port || 3000}</dd>
                    </div>
                  </dl>
                </Card>

                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">
                    Build Configuration
                  </h3>
                  <dl className="space-y-4">
                    <div>
                      <dt className="text-sm text-surface-400">Build Command</dt>
                      <dd className="mt-1 rounded-lg bg-surface-800 px-3 py-2 font-mono text-sm text-foreground">
                        {project.build_command || "npm run build"}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-sm text-surface-400">Start Command</dt>
                      <dd className="mt-1 rounded-lg bg-surface-800 px-3 py-2 font-mono text-sm text-foreground">
                        {project.start_command || "npm start"}
                      </dd>
                    </div>
                  </dl>
                </Card>
              </div>
            )}

            {activeTab === "builds" && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-semibold text-foreground">
                    Build History
                  </h3>
                  <button
                    onClick={handleClearBuildLogs}
                    disabled={triggeringBuild}
                    className="inline-flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
                  >
                    {triggeringBuild ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Trash2 className="h-4 w-4" />
                    )}
                    Clear Build History Logs
                  </button>
                </div>

                {buildsLoading ? (
                  <Card variant="elevated">
                    <div className="flex items-center justify-center py-12">
                      <Loader2 className="h-6 w-6 animate-spin text-primary" />
                    </div>
                  </Card>
                ) : builds.length === 0 ? (
                  <Card variant="elevated">
                    <div className="text-center py-12">
                      <RefreshCw className="mx-auto mb-4 h-12 w-12 text-surface-600" />
                      <p className="text-surface-400">No builds yet</p>
                      <p className="mt-1 text-sm text-surface-500">
                        Trigger a build to see build history
                      </p>
                    </div>
                  </Card>
                ) : (
                  <div className="space-y-3">
                    {builds.map((build) => (
                      <div key={build.id} className="space-y-3">
                        <BuildCard
                          build={build}
                          onClick={() =>
                            setExpandedBuildId(
                              expandedBuildId === build.id ? null : build.id
                            )
                          }
                          isExpanded={expandedBuildId === build.id}
                        />
                        {expandedBuildId === build.id && accessToken && (
                          <Card variant="elevated" className="ml-4">
                            <BuildLogs
                              key={`${build.id}-${logsClearedTimestamp}`} // Force remount when logs are cleared
                              buildId={build.id}
                              projectId={projectId}
                              token={accessToken}
                              buildStatus={build.status}
                            />
                          </Card>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === "settings" && (
              <div className="space-y-6">
                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">
                    General Settings
                  </h3>
                  <p className="text-surface-400">
                    Project settings configuration coming soon.
                  </p>
                </Card>

                <Card variant="elevated" className="border-accent-rose/30">
                  <h3 className="mb-4 text-lg font-semibold text-accent-rose">
                    Danger Zone
                  </h3>
                  <p className="mb-4 text-sm text-surface-400">
                    Once you delete a project, there is no going back. Please be certain.
                  </p>
                  <button
                    onClick={() => setShowDeleteConfirm(true)}
                    className="inline-flex items-center gap-2 rounded-lg border border-accent-rose/50 px-4 py-2 text-sm font-medium text-accent-rose transition-colors hover:bg-accent-rose/10"
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete Project
                  </button>
                </Card>
              </div>
            )}

            {activeTab === "secrets" && (
              <Card variant="elevated">
                <h3 className="mb-4 text-lg font-semibold text-foreground">
                  Environment Variables
                </h3>
                <div className="text-center py-12">
                  <Key className="mx-auto mb-4 h-12 w-12 text-surface-600" />
                  <p className="text-surface-400">No secrets configured</p>
                  <p className="mt-1 text-sm text-surface-500">
                    Add environment variables for your application
                  </p>
                </div>
              </Card>
            )}
              </>
            )}
          </div>
        </main>
      </div>

      {/* Delete Confirmation Modal */}
      {showDeleteConfirm && (
        <>
          <div
            className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
            onClick={() => setShowDeleteConfirm(false)}
          />
          <div className="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 transform">
            <Card variant="elevated" className="border-accent-rose/30">
              <div className="p-6">
                <h3 className="mb-2 text-lg font-semibold text-accent-rose">
                  Delete Project
                </h3>
                <p className="mb-6 text-sm text-surface-400">
                  Are you sure you want to delete this project? This action cannot be undone. All builds, deployments, and logs will be permanently deleted.
                </p>
                <div className="flex items-center justify-end gap-3">
                  <button
                    onClick={() => setShowDeleteConfirm(false)}
                    disabled={isDeleting}
                    className="rounded-lg border border-surface-700 px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surface-800 disabled:opacity-50"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleDelete}
                    disabled={isDeleting}
                    className="inline-flex items-center gap-2 rounded-lg bg-accent-rose px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-rose/90 disabled:opacity-50"
                  >
                    {isDeleting ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Deleting...
                      </>
                    ) : (
                      <>
                        <Trash2 className="h-4 w-4" />
                        Delete Project
                      </>
                    )}
                  </button>
                </div>
              </div>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

