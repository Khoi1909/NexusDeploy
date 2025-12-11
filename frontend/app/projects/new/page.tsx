"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import { projectApi, CreateProjectRequest } from "@/lib/api/projects";
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

  const [formData, setFormData] = useState({
    name: "",
    repo_url: "",
    branch: "main",
    preset: "nodejs",
    build_command: "npm run build",
    start_command: "npm start",
    env_vars: [{ key: "", value: "" }],
  });

  const handleInputChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

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
                      GitHub Repository URL
                    </label>
                    <div className="relative">
                      <GitBranch className="absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-surface-500" />
                      <input
                        type="text"
                        value={formData.repo_url}
                        onChange={(e) => handleInputChange("repo_url", e.target.value)}
                        placeholder="https://github.com/username/repo"
                        className="w-full rounded-lg border border-surface-700 bg-surface-900 py-3 pl-10 pr-4 text-foreground placeholder:text-surface-500 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                      />
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

