"use client";

import { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import { projectApi, CreateProjectRequest, Repository } from "@/lib/api/projects";
import {
  ArrowLeft,
  GitBranch,
  Folder,
  Terminal,
  Play,
  Plus,
  Trash2,
  Loader2,
  CheckCircle,
  Search,
} from "lucide-react";
import Link from "next/link";

const presets = [
  { id: "nodejs", name: "Node.js", description: "JavaScript/TypeScript runtime" },
  { id: "go", name: "Go", description: "Go programming language" },
  { id: "python", name: "Python", description: "Python runtime" },
  { id: "static", name: "Static", description: "Static HTML/CSS/JS" },
  { id: "docker", name: "Docker", description: "Custom Dockerfile" },
];

export default function NewProjectPage() {
  const router = useRouter();
  const { accessToken } = useAuthStore();
  const [step, setStep] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Repository selection state
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(false);
  const [selectedRepo, setSelectedRepo] = useState<Repository | null>(null);
  const [repoSearchQuery, setRepoSearchQuery] = useState("");
  const [showRepoDropdown, setShowRepoDropdown] = useState(false);
  const repoDropdownRef = useRef<HTMLDivElement>(null);

  const [formData, setFormData] = useState({
    name: "",
    repo_url: "",
    branch: "main",
    preset: "nodejs",
    build_command: "npm run build",
    start_command: "npm start",
    env_vars: [{ key: "", value: "" }],
    github_repo_id: 0,
    is_private: false,
  });

  // Fetch repositories on mount
  useEffect(() => {
    const fetchRepos = async () => {
      if (!accessToken) return;

      setLoadingRepos(true);
      try {
        const repos = await projectApi.listRepositories(accessToken);
        setRepositories(repos);
      } catch (err: any) {
        console.error("Failed to load repositories:", err);
        // Don't set error, allow manual URL input as fallback
      } finally {
        setLoadingRepos(false);
      }
    };

    fetchRepos();
  }, [accessToken]);

  // Handle click outside to close dropdown
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (repoDropdownRef.current && !repoDropdownRef.current.contains(event.target as Node)) {
        setShowRepoDropdown(false);
      }
    };

    if (showRepoDropdown) {
      document.addEventListener("mousedown", handleClickOutside);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [showRepoDropdown]);

  const handleInputChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  // Handle repository selection
  const handleRepoSelect = (repo: Repository) => {
    setSelectedRepo(repo);
    setFormData((prev) => ({
      ...prev,
      repo_url: repo.clone_url,
      branch: repo.default_branch,
      github_repo_id: repo.id,
      is_private: repo.private || repo.is_private || false,
    }));
    setShowRepoDropdown(false);
    setRepoSearchQuery("");
  };

  // Filter repositories based on search query
  const filteredRepos = repositories.filter((repo) =>
    repo.name.toLowerCase().includes(repoSearchQuery.toLowerCase()) ||
    repo.full_name.toLowerCase().includes(repoSearchQuery.toLowerCase()) ||
    (repo.description && repo.description.toLowerCase().includes(repoSearchQuery.toLowerCase()))
  );

  const handleEnvVarChange = (index: number, field: "key" | "value", value: string) => {
    setFormData((prev) => {
      const newEnvVars = [...prev.env_vars];
      newEnvVars[index] = { ...newEnvVars[index], [field]: value };
      return { ...prev, env_vars: newEnvVars };
    });
  };

  const addEnvVar = () => {
    setFormData((prev) => ({
      ...prev,
      env_vars: [...prev.env_vars, { key: "", value: "" }],
    }));
  };

  const removeEnvVar = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      env_vars: prev.env_vars.filter((_, i) => i !== index),
    }));
  };

  const handleSubmit = async () => {
    if (!accessToken) {
      setError("Not authenticated. Please log in again.");
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const payload: CreateProjectRequest = {
        name: formData.name,
        repo_url: formData.repo_url,
        branch: formData.branch,
        preset: formData.preset,
        build_command: formData.build_command,
        start_command: formData.start_command,
        github_repo_id: formData.github_repo_id || undefined,
        is_private: formData.is_private,
      };

      await projectApi.createProject(accessToken, payload);
      router.push("/dashboard");
    } catch (err: any) {
      console.error("Failed to create project:", err);
      const errorMessage = err.message || err.code || "Failed to create project";
      setError(errorMessage);
    } finally {
      setIsSubmitting(false);
    }
  };

  const isStep1Valid = formData.name && formData.repo_url;
  const isStep2Valid = formData.preset;

  return (
    <div className="relative min-h-screen bg-background">
      <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
      <Navbar />

      <div className="flex">
        <Sidebar />

        <main className="flex-1 lg:ml-64 pt-16">
          <div className="mx-auto max-w-3xl px-4 py-8 sm:px-6 lg:px-8">
            {/* Header */}
            <div className="mb-8">
              <Link
                href="/dashboard"
                className="mb-4 inline-flex items-center gap-2 text-sm text-surface-400 transition-colors hover:text-foreground"
              >
                <ArrowLeft className="h-4 w-4" />
                Back to Dashboard
              </Link>
              <h1 className="text-2xl font-bold text-foreground sm:text-3xl">
                Create New Project
              </h1>
              <p className="mt-1 text-surface-400">
                Deploy your application in minutes
              </p>
            </div>

            {/* Progress Steps */}
            <div className="mb-8 flex items-center gap-4">
              {[1, 2, 3].map((s) => (
                <div key={s} className="flex items-center gap-2">
                  <div
                    className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors ${
                      step >= s
                        ? "bg-primary text-white"
                        : "bg-surface-800 text-surface-400"
                    }`}
                  >
                    {step > s ? <CheckCircle className="h-4 w-4" /> : s}
                  </div>
                  {s < 3 && (
                    <div
                      className={`h-px w-12 transition-colors ${
                        step > s ? "bg-primary" : "bg-surface-700"
                      }`}
                    />
                  )}
                </div>
              ))}
            </div>

            {error && (
              <div className="mb-6 rounded-lg border border-accent-rose/30 bg-accent-rose/10 p-4 text-accent-rose">
                {error}
              </div>
            )}

            {/* Step 1: Basic Info */}
            {step === 1 && (
              <Card variant="elevated" className="animate-fade-in">
                <h2 className="mb-6 text-lg font-semibold text-foreground">
                  Repository Details
                </h2>

                <div className="space-y-6">
                  <div>
                    <label className="mb-2 block text-sm font-medium text-foreground">
                      Project Name
                    </label>
                    <div className="relative">
                      <Folder className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500" />
                      <input
                        type="text"
                        value={formData.name}
                        onChange={(e) => handleInputChange("name", e.target.value)}
                        placeholder="my-awesome-project"
                        className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-foreground">
                      GitHub Repository
                    </label>
                    <div className="relative" ref={repoDropdownRef}>
                      {loadingRepos ? (
                        <div className="flex items-center gap-3 rounded-lg border border-surface-700 bg-surface-900 py-3 px-4">
                          <Loader2 className="h-5 w-5 animate-spin text-surface-400" />
                          <span className="text-sm text-surface-400">Loading repositories...</span>
                        </div>
                      ) : (
                        <>
                          <div className="relative">
                            <GitBranch className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500 z-10" />
                            <input
                              type="text"
                              value={selectedRepo ? selectedRepo.full_name : repoSearchQuery}
                              onChange={(e) => {
                                setRepoSearchQuery(e.target.value);
                                setShowRepoDropdown(true);
                                if (!e.target.value) {
                                  setSelectedRepo(null);
                                  handleInputChange("repo_url", "");
                                  handleInputChange("branch", "main");
                                }
                              }}
                              onFocus={() => setShowRepoDropdown(true)}
                              placeholder="Search or select a repository..."
                              className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                            />
                            {selectedRepo && (
                              <button
                                type="button"
                                onClick={() => {
                                  setSelectedRepo(null);
                                  setRepoSearchQuery("");
                                  handleInputChange("repo_url", "");
                                  handleInputChange("branch", "main");
                                }}
                                className="absolute right-3 top-1/2 -translate-y-1/2 text-surface-400 hover:text-foreground"
                              >
                                ×
                              </button>
                            )}
                          </div>

                          {/* Dropdown */}
                          {showRepoDropdown && (repoSearchQuery || !selectedRepo) && (
                            <div className="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-surface-700 bg-surface-900 shadow-xl">
                              {filteredRepos.length === 0 ? (
                                <div className="p-4 text-center text-sm text-surface-400">
                                  {repositories.length === 0
                                    ? "No repositories found"
                                    : "No repositories match your search"}
                                </div>
                              ) : (
                                <div className="py-2">
                                  {filteredRepos.slice(0, 10).map((repo) => (
                                    <button
                                      key={repo.id}
                                      type="button"
                                      onClick={() => handleRepoSelect(repo)}
                                      className="w-full px-4 py-3 text-left hover:bg-surface-800 transition-colors"
                                    >
                                      <div className="flex items-center justify-between">
                                        <div className="flex-1 min-w-0">
                                          <div className="flex items-center gap-2">
                                            <span className="font-medium text-foreground truncate">
                                              {repo.full_name}
                                            </span>
                                            {repo.private && (
                                              <span className="text-xs text-surface-500">Private</span>
                                            )}
                                          </div>
                                          {repo.description && (
                                            <p className="mt-1 text-sm text-surface-400 truncate">
                                              {repo.description}
                                            </p>
                                          )}
                                          {repo.default_branch && (
                                            <p className="mt-1 text-xs text-surface-500">
                                              Branch: {repo.default_branch}
                                            </p>
                                          )}
                                        </div>
                                      </div>
                                    </button>
                                  ))}
                                </div>
                              )}
                            </div>
                          )}

                          {/* Manual URL input fallback */}
                          {selectedRepo && (
                            <div className="mt-2 text-xs text-surface-400">
                              Selected: {selectedRepo.clone_url}
                            </div>
                          )}
                          <div className="mt-2">
                            <button
                              type="button"
                              onClick={() => {
                                setSelectedRepo(null);
                                setRepoSearchQuery("");
                                setShowRepoDropdown(false);
                              }}
                              className="text-xs text-primary hover:underline"
                            >
                              Or enter repository URL manually
                            </button>
                          </div>
                          {!selectedRepo && (
                            <input
                              type="text"
                              value={formData.repo_url}
                              onChange={(e) => handleInputChange("repo_url", e.target.value)}
                              placeholder="https://github.com/username/repo"
                              className="mt-2 w-full rounded-lg border border-surface-700 bg-surface-900 px-4 py-2 text-sm text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                            />
                          )}
                        </>
                      )}
                    </div>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-foreground">
                      Branch
                    </label>
                    <input
                      type="text"
                      value={formData.branch}
                      onChange={(e) => handleInputChange("branch", e.target.value)}
                      placeholder="main"
                      className="w-full rounded-lg border border-surface-700 bg-surface-900 px-4 py-3 text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                    />
                  </div>
                </div>

                <div className="mt-8 flex justify-end">
                  <button
                    onClick={() => setStep(2)}
                    disabled={!isStep1Valid}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Continue
                  </button>
                </div>
              </Card>
            )}

            {/* Step 2: Build Settings */}
            {step === 2 && (
              <Card variant="elevated" className="animate-fade-in">
                <h2 className="mb-6 text-lg font-semibold text-foreground">
                  Build Configuration
                </h2>

                <div className="space-y-6">
                  <div>
                    <label className="mb-3 block text-sm font-medium text-foreground">
                      Framework Preset
                    </label>
                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                      {presets.map((preset) => (
                        <button
                          key={preset.id}
                          onClick={() => handleInputChange("preset", preset.id)}
                          className={`rounded-lg border p-4 text-left transition-all ${
                            formData.preset === preset.id
                              ? "border-primary bg-primary/10"
                              : "border-surface-700 bg-surface-800/50 hover:border-surface-600"
                          }`}
                        >
                          <p className="font-medium text-foreground">{preset.name}</p>
                          <p className="mt-1 text-xs text-surface-400">
                            {preset.description}
                          </p>
                        </button>
                      ))}
                    </div>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-foreground">
                      Build Command
                    </label>
                    <div className="relative">
                      <Terminal className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500" />
                      <input
                        type="text"
                        value={formData.build_command}
                        onChange={(e) => handleInputChange("build_command", e.target.value)}
                        placeholder="npm run build"
                        className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 font-mono text-sm text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-foreground">
                      Start Command
                    </label>
                    <div className="relative">
                      <Play className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500" />
                      <input
                        type="text"
                        value={formData.start_command}
                        onChange={(e) => handleInputChange("start_command", e.target.value)}
                        placeholder="npm start"
                        className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 font-mono text-sm text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                    </div>
                  </div>
                </div>

                <div className="mt-8 flex justify-between">
                  <button
                    onClick={() => setStep(1)}
                    className="rounded-lg border border-surface-700 px-6 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-surface-800"
                  >
                    Back
                  </button>
                  <button
                    onClick={() => setStep(3)}
                    disabled={!isStep2Valid}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Continue
                  </button>
                </div>
              </Card>
            )}

            {/* Step 3: Environment Variables */}
            {step === 3 && (
              <Card variant="elevated" className="animate-fade-in">
                <h2 className="mb-6 text-lg font-semibold text-foreground">
                  Environment Variables
                </h2>
                <p className="mb-6 text-sm text-surface-400">
                  Add environment variables for your application. These will be encrypted
                  and securely stored.
                </p>

                <div className="space-y-3">
                  {formData.env_vars.map((env, index) => (
                    <div key={index} className="flex items-center gap-3">
                      <input
                        type="text"
                        value={env.key}
                        onChange={(e) => handleEnvVarChange(index, "key", e.target.value)}
                        placeholder="KEY"
                        className="flex-1 rounded-lg border border-surface-700 bg-surface-900 px-4 py-2.5 font-mono text-sm text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                      <input
                        type="text"
                        value={env.value}
                        onChange={(e) => handleEnvVarChange(index, "value", e.target.value)}
                        placeholder="value"
                        className="flex-1 rounded-lg border border-surface-700 bg-surface-900 px-4 py-2.5 font-mono text-sm text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
                      <button
                        onClick={() => removeEnvVar(index)}
                        className="rounded-lg p-2.5 text-surface-400 transition-colors hover:bg-surface-800 hover:text-accent-rose"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))}
                </div>

                <button
                  onClick={addEnvVar}
                  className="mt-4 inline-flex items-center gap-2 text-sm text-surface-400 transition-colors hover:text-foreground"
                >
                  <Plus className="h-4 w-4" />
                  Add Variable
                </button>

                <div className="mt-8 flex justify-between">
                  <button
                    onClick={() => setStep(2)}
                    className="rounded-lg border border-surface-700 px-6 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-surface-800"
                  >
                    Back
                  </button>
                  <button
                    onClick={handleSubmit}
                    disabled={isSubmitting}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {isSubmitting ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Creating...
                      </>
                    ) : (
                      <>
                        <Plus className="h-4 w-4" />
                        Create Project
                      </>
                    )}
                  </button>
                </div>
              </Card>
            )}
          </div>
        </main>
      </div>
    </div>
  );
}

