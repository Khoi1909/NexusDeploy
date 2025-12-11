"use client";

import Link from "next/link";
import { LightPillars } from "@/components/ui/LightPillars";
import { useAuthStore } from "@/lib/store/authStore";
import { ArrowRight, GitBranch, Zap, Shield, Cpu, LayoutDashboard } from "lucide-react";

export default function HomePage() {
  const { isAuthenticated } = useAuthStore();

  return (
    <div className="relative min-h-screen overflow-hidden bg-background">
      {/* Light Pillars Background */}
      <LightPillars pillarsCount={7} />

      {/* Noise overlay */}
      <div className="absolute inset-0 bg-noise opacity-30 pointer-events-none" />

      {/* Content */}
      <div className="relative z-10">
        {/* Header */}
        <header className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <img
                src="/logo.png"
                alt="NexusDeploy"
                className="h-10 w-10 rounded-xl object-contain"
              />
              <span className="text-xl font-bold text-foreground">NexusDeploy</span>
            </div>
            {isAuthenticated ? (
              <Link
                href="/dashboard"
                className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-medium text-white transition-all duration-200 hover:bg-primary-600"
              >
                <LayoutDashboard className="h-4 w-4" />
                Dashboard
              </Link>
            ) : (
              <Link
                href="/login"
                className="rounded-lg bg-surface-800 px-5 py-2.5 text-sm font-medium text-foreground transition-all duration-200 hover:bg-surface-700"
              >
                Sign In
              </Link>
            )}
          </div>
        </header>

        {/* Hero Section */}
        <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex min-h-[calc(100vh-200px)] flex-col items-center justify-center text-center">
            {/* Badge */}
            <div className="mb-8 animate-fade-in">
              <span className="inline-flex items-center gap-2 rounded-full border border-primary/30 bg-primary/10 px-4 py-1.5 text-sm font-medium text-primary">
                <Zap className="h-4 w-4" />
                AI-Powered CI/CD Platform
              </span>
            </div>

            {/* Heading */}
            <h1 className="mb-6 max-w-4xl text-5xl font-bold tracking-tight text-foreground sm:text-6xl lg:text-7xl animate-slide-up">
              Deploy with{" "}
              <span className="text-gradient">Intelligence</span>
            </h1>

            {/* Subheading */}
            <p className="mb-10 max-w-2xl text-lg text-surface-400 sm:text-xl animate-slide-up" style={{ animationDelay: "0.1s" }}>
              The next-generation PaaS platform with AI-powered error analysis. 
              Connect your GitHub, push your code, and let NexusDeploy handle the rest.
            </p>

            {/* CTA Buttons */}
            <div className="flex flex-col gap-4 sm:flex-row animate-slide-up" style={{ animationDelay: "0.2s" }}>
              <Link
                href="/login"
                className="group inline-flex items-center justify-center gap-2 rounded-xl bg-primary px-8 py-4 text-base font-semibold text-white shadow-lg shadow-primary/30 transition-all duration-300 hover:bg-primary-600 hover:shadow-xl hover:shadow-primary/40 hover:-translate-y-0.5"
              >
                <GitBranch className="h-5 w-5" />
                Login with GitHub
                <ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-1" />
              </Link>
              <a
                href="https://github.com/nexusdeploy"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-xl border border-surface-700 bg-surface-900/50 px-8 py-4 text-base font-semibold text-foreground backdrop-blur-sm transition-all duration-300 hover:border-surface-600 hover:bg-surface-800/50"
              >
                View Documentation
              </a>
            </div>

            {/* Features Grid */}
            <div className="mt-24 grid w-full max-w-4xl grid-cols-1 gap-6 sm:grid-cols-3 animate-slide-up" style={{ animationDelay: "0.3s" }}>
              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70">
                <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                  <Cpu className="h-6 w-6 text-primary" />
                </div>
                <h3 className="mb-2 text-lg font-semibold text-foreground">AI Error Analysis</h3>
                <p className="text-sm text-surface-400">
                  Intelligent analysis of build errors with actionable suggestions.
                </p>
              </div>

              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70">
                <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-lg bg-accent-cyan/10">
                  <GitBranch className="h-6 w-6 text-accent-cyan" />
                </div>
                <h3 className="mb-2 text-lg font-semibold text-foreground">GitHub Integration</h3>
                <p className="text-sm text-surface-400">
                  Seamless connection with your repositories and automatic deployments.
                </p>
              </div>

              <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70">
                <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-lg bg-accent-emerald/10">
                  <Shield className="h-6 w-6 text-accent-emerald" />
                </div>
                <h3 className="mb-2 text-lg font-semibold text-foreground">Secure by Default</h3>
                <p className="text-sm text-surface-400">
                  End-to-end encryption for your secrets and environment variables.
                </p>
              </div>
            </div>
          </div>
        </main>

        {/* Footer */}
        <footer className="border-t border-surface-800 mt-20">
          <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
            <div className="flex flex-col items-center justify-between gap-4 sm:flex-row">
              <p className="text-sm text-surface-500">
                2024 NexusDeploy. Built for developers.
              </p>
              <div className="flex items-center gap-6">
                <a href="#" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                  Documentation
                </a>
                <a href="#" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                  GitHub
                </a>
                <a href="#" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                  Support
                </a>
              </div>
            </div>
          </div>
        </footer>
      </div>
    </div>
  );
}
