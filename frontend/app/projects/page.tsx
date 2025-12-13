"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useProjectStore } from "@/lib/store/projectStore";
import { useAuthStore } from "@/lib/store/authStore";
import { projectApi } from "@/lib/api/projects";
import { useRouter } from "next/navigation";
import {
  Plus,
  GitBranch,
  Clock,
  ExternalLink,
  RefreshCw,
  Server,
  AlertTriangle,
  Search,
} from "lucide-react";

export default function ProjectsPage() {
  const router = useRouter();
  const { projects, setProjects } = useProjectStore();
  const { accessToken, logout, isLoading: authLoading, isAuthenticated } = useAuthStore();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

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
      // Only show loading if we don't have cached projects
      if (!hasCachedProjects) {
        setIsLoading(true);
      }
      setError(null);
      
      try {
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

  // Filter projects based on search query
  const filteredProjects = projects.filter((project) =>
    project.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    project.repo_url.toLowerCase().includes(searchQuery.toLowerCase())
  );

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
                <h1 className="text-2xl font-bold text-foreground sm:text-3xl">Projects</h1>
                <p className="mt-1 text-surface-400">
                  View and manage all your projects
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

            {/* Search */}
            <div className="mb-6">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search projects by name or repository URL..."
                  className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </div>
            </div>

            {/* Projects List */}
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
              <Card variant="elevated">
                <div className="rounded-xl border border-accent-rose/30 bg-accent-rose/10 p-6 text-center">
                  <AlertTriangle className="mx-auto mb-3 h-8 w-8 text-accent-rose" />
                  <p className="text-foreground">{error}</p>
                </div>
              </Card>
            ) : filteredProjects.length === 0 ? (
              <Card variant="elevated">
                <div className="rounded-xl border border-dashed border-surface-700 bg-surface-900/30 p-12 text-center">
                  <Server className="mx-auto mb-4 h-12 w-12 text-surface-600" />
                  <h3 className="mb-2 text-lg font-medium text-foreground">
                    {searchQuery ? "No projects found" : "No projects yet"}
                  </h3>
                  <p className="mb-6 text-surface-400">
                    {searchQuery
                      ? "Try adjusting your search query"
                      : "Get started by creating your first project"}
                  </p>
                  {!searchQuery && (
                    <Link
                      href="/projects/new"
                      className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600"
                    >
                      <Plus className="h-4 w-4" />
                      Create Project
                    </Link>
                  )}
                </div>
              </Card>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {filteredProjects.map((project) => (
                  <Link key={project.id} href={`/projects/${project.id}`}>
                    <Card variant="elevated" className="h-full cursor-pointer transition-all hover:border-primary/50">
                      <div className="flex h-full flex-col">
                        <div className="mb-4 flex items-start justify-between">
                          <h3 className="text-lg font-semibold text-foreground">
                            {project.name}
                          </h3>
                          <ExternalLink className="h-4 w-4 text-surface-500 transition-colors hover:text-foreground" />
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
                          <div className="text-sm text-surface-400">
                            <span className="truncate block">{project.repo_url}</span>
                          </div>
                        </div>

                        <div className="mt-4 flex items-center justify-between border-t border-surface-800 pt-4">
                          <span className="rounded-md bg-surface-800 px-2 py-1 text-xs font-medium text-surface-300">
                            {project.preset}
                          </span>
                        </div>
                      </div>
                    </Card>
                  </Link>
                ))}
              </div>
            )}
          </div>
        </main>
      </div>
    </div>
  );
}
