"use client";

import { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import { projectApi, Project } from "@/lib/api/projects";
import { buildsApi, Build } from "@/lib/api/builds";
import { deploymentsApi, Deployment } from "@/lib/api/deployments";
import { useRouter } from "next/navigation";
import {
  Activity,
  GitBranch,
  RefreshCw,
  Play,
  Square,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
  Filter,
  Calendar,
  Loader2,
} from "lucide-react";

type EventType = "build" | "deployment" | "project";

interface ActivityEvent {
  id: string;
  type: EventType;
  timestamp: Date;
  projectId: string;
  projectName: string;
  status?: string;
  label: string;
  icon: any;
  color: string;
  bgColor: string;
  link?: string;
}

export default function ActivityPage() {
  const router = useRouter();
  const { accessToken, isLoading: authLoading, isAuthenticated } = useAuthStore();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string | "all">("all");
  const [selectedEventType, setSelectedEventType] = useState<EventType | "all">("all");
  const [dateRange, setDateRange] = useState<{ start?: Date; end?: Date }>({});

  useEffect(() => {
    if (authLoading) {
      return;
    }

    if (!isAuthenticated && !accessToken) {
      setIsLoading(false);
      router.push("/");
      return;
    }

    if (!accessToken) {
      return;
    }

    const fetchData = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;

        // Fetch all projects
        const projectsData = await projectApi.listProjects(currentToken);
        setProjects(projectsData);
      } catch (err: any) {
        console.error("Failed to fetch activity data:", err);
        const status = (err as any).status;
        const errorMessage = err.message || err.code || "Failed to fetch activity data";

        if (status === 401 || errorMessage.toLowerCase().includes("unauthorized")) {
          router.push("/");
          return;
        }

        setError(errorMessage);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, [accessToken, authLoading, isAuthenticated, router]);

  // Fetch all builds and deployments for activity events
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [eventsLoading, setEventsLoading] = useState(false);

  useEffect(() => {
    if (!accessToken || isLoading || projects.length === 0 || authLoading) {
      return;
    }

    const fetchActivityEvents = async () => {
      setEventsLoading(true);
      try {
        const currentToken = useAuthStore.getState().accessToken;
        if (!currentToken) return;

        const allEvents: ActivityEvent[] = [];

        // Fetch builds and deployments for each project in parallel
        const eventPromises = projects.map(async (project) => {
          try {
            // Fetch builds
            const buildsResponse = await buildsApi.listBuilds(currentToken, project.id, 1, 50);
            const builds = buildsResponse.builds || [];

            // Fetch deployment
            let deployment: Deployment | null = null;
            try {
              deployment = await deploymentsApi.getDeploymentStatus(currentToken, project.id);
            } catch {
              // Deployment might not exist
            }

            // Add project created event
            allEvents.push({
              id: `project-${project.id}`,
              type: "project",
              timestamp: new Date(project.created_at),
              projectId: project.id,
              projectName: project.name,
              label: "Project created",
              icon: GitBranch,
              color: "text-purple-500",
              bgColor: "bg-purple-500/10",
              link: `/projects/${project.id}`,
            });

            // Add build events
            builds.forEach((build) => {
              const timestamp = build.started_at ? new Date(build.started_at) : new Date(build.created_at);
              let icon = RefreshCw;
              let color = "text-blue-500";
              let bgColor = "bg-blue-500/10";
              let label = "Build started";

              if (build.status === "success") {
                icon = CheckCircle;
                color = "text-accent-emerald";
                bgColor = "bg-accent-emerald/10";
                label = "Build succeeded";
              } else if (build.status === "failed" || build.status === "deploy_failed") {
                icon = XCircle;
                color = "text-accent-rose";
                bgColor = "bg-accent-rose/10";
                label = "Build failed";
              }

              allEvents.push({
                id: `build-${build.id}`,
                type: "build",
                timestamp,
                projectId: project.id,
                projectName: project.name,
                status: build.status,
                label,
                icon,
                color,
                bgColor,
                link: `/projects/${project.id}?build=${build.id}`,
              });
            });

            // Add deployment events
            if (deployment) {
              const deployTimestamp = new Date(); // Use current time as approximation
              let icon = Play;
              let color = "text-accent-emerald";
              let bgColor = "bg-accent-emerald/10";
              let label = "Deployment started";

              if (deployment.status === "running") {
                icon = CheckCircle;
                label = "Deployment running";
              } else if (deployment.status === "stopped") {
                icon = Square;
                color = "text-surface-400";
                bgColor = "bg-surface-800";
                label = "Deployment stopped";
              } else if (deployment.status === "failed") {
                icon = XCircle;
                color = "text-accent-rose";
                bgColor = "bg-accent-rose/10";
                label = "Deployment failed";
              }

              allEvents.push({
                id: `deployment-${deployment.id}`,
                type: "deployment",
                timestamp: deployTimestamp,
                projectId: project.id,
                projectName: project.name,
                status: deployment.status,
                label,
                icon,
                color,
                bgColor,
                link: `/projects/${project.id}`,
              });
            }
          } catch (err) {
            console.error(`Failed to fetch events for project ${project.id}:`, err);
          }
        });

        await Promise.all(eventPromises);

        // Sort events by timestamp (newest first)
        allEvents.sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());

        setEvents(allEvents);
      } catch (err: any) {
        console.error("Failed to fetch activity events:", err);
        setError(err.message || "Failed to fetch activity events");
      } finally {
        setEventsLoading(false);
      }
    };

    fetchActivityEvents();
  }, [accessToken, authLoading, isLoading, projects]);

  // Filter events
  const filteredEvents = useMemo(() => {
    return events.filter((event) => {
      // Filter by project
      if (selectedProjectId !== "all" && event.projectId !== selectedProjectId) {
        return false;
      }

      // Filter by event type
      if (selectedEventType !== "all" && event.type !== selectedEventType) {
        return false;
      }

      // Filter by date range
      if (dateRange.start && event.timestamp < dateRange.start) {
        return false;
      }
      if (dateRange.end) {
        const endDate = new Date(dateRange.end);
        endDate.setHours(23, 59, 59, 999);
        if (event.timestamp > endDate) {
          return false;
        }
      }

      return true;
    });
  }, [events, selectedProjectId, selectedEventType, dateRange]);

  const formatTimestamp = (date: Date) => {
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return "Just now";
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return date.toLocaleDateString("vi-VN", { day: "numeric", month: "short", year: "numeric" });
  };

  return (
    <div className="relative min-h-screen bg-background">
      <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
      <Navbar />

      <div className="flex">
        <Sidebar />

        <main className="flex-1 lg:ml-64 pt-16">
          <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
            {/* Header */}
            <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h1 className="text-2xl font-bold text-foreground sm:text-3xl">Activity</h1>
                <p className="mt-1 text-surface-400">
                  View all activity across your projects
                </p>
              </div>
            </div>

            {/* Filters */}
            <Card variant="elevated" className="mb-6">
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-2">
                  <Filter className="h-4 w-4 text-surface-400" />
                  <span className="text-sm font-medium text-surface-400">Filters:</span>
                </div>

                {/* Project filter */}
                <select
                  value={selectedProjectId}
                  onChange={(e) => setSelectedProjectId(e.target.value)}
                  className="rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                >
                  <option value="all">All Projects</option>
                  {projects.map((project) => (
                    <option key={project.id} value={project.id}>
                      {project.name}
                    </option>
                  ))}
                </select>

                {/* Event type filter */}
                <select
                  value={selectedEventType}
                  onChange={(e) => setSelectedEventType(e.target.value as EventType | "all")}
                  className="rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                >
                  <option value="all">All Events</option>
                  <option value="project">Projects</option>
                  <option value="build">Builds</option>
                  <option value="deployment">Deployments</option>
                </select>

                {/* Date range (optional, simplified) */}
                <button
                  onClick={() => {
                    const today = new Date();
                    const weekAgo = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000);
                    setDateRange({ start: weekAgo, end: today });
                  }}
                  className="inline-flex items-center gap-2 rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-foreground transition-colors hover:bg-surface-800"
                >
                  <Calendar className="h-4 w-4" />
                  Last 7 days
                </button>

                {(dateRange.start || dateRange.end) && (
                  <button
                    onClick={() => setDateRange({})}
                    className="text-sm text-surface-400 hover:text-foreground"
                  >
                    Clear date filter
                  </button>
                )}
              </div>
            </Card>

            {/* Activity Timeline */}
            {error ? (
              <Card variant="elevated">
                <div className="rounded-xl border border-accent-rose/30 bg-accent-rose/10 p-6 text-center">
                  <AlertTriangle className="mx-auto mb-3 h-8 w-8 text-accent-rose" />
                  <p className="text-foreground">{error}</p>
                </div>
              </Card>
            ) : eventsLoading || isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : filteredEvents.length === 0 ? (
              <Card variant="elevated">
                <div className="rounded-xl border border-dashed border-surface-700 bg-surface-900/30 p-12 text-center">
                  <Activity className="mx-auto mb-4 h-12 w-12 text-surface-600" />
                  <h3 className="mb-2 text-lg font-medium text-foreground">No activity found</h3>
                  <p className="text-surface-400">
                    {events.length === 0
                      ? "Start by creating a project and triggering a build"
                      : "Try adjusting your filters"}
                  </p>
                </div>
              </Card>
            ) : (
              <div className="relative">
                {/* Timeline line */}
                <div className="absolute left-8 top-0 bottom-0 w-0.5 bg-surface-800" />

                {/* Events */}
                <div className="space-y-6">
                  {filteredEvents.map((event, index) => {
                    const Icon = event.icon;
                    return (
                      <div key={event.id} className="relative flex gap-4">
                        {/* Timeline dot */}
                        <div
                          className={`relative z-10 flex h-16 w-16 shrink-0 items-center justify-center rounded-full border-4 border-background ${event.bgColor}`}
                        >
                          <Icon className={`h-6 w-6 ${event.color}`} />
                        </div>

                        {/* Event card */}
                        <div className="flex-1 pb-6">
                          <Link href={event.link || "#"}>
                            <Card
                              variant="elevated"
                              className="transition-colors hover:border-primary/50"
                            >
                              <div className="flex items-start justify-between">
                                <div className="flex-1">
                                  <div className="flex items-center gap-3">
                                    <span className={`text-sm font-medium ${event.color}`}>
                                      {event.label}
                                    </span>
                                    <span className="text-xs text-surface-500">
                                      {formatTimestamp(event.timestamp)}
                                    </span>
                                  </div>
                                  <p className="mt-1 text-sm text-foreground">{event.projectName}</p>
                                  {event.status && (
                                    <span className="mt-2 inline-block rounded-md bg-surface-800 px-2 py-1 text-xs text-surface-400">
                                      {event.status}
                                    </span>
                                  )}
                                </div>
                              </div>
                            </Card>
                          </Link>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </main>
      </div>
    </div>
  );
}
