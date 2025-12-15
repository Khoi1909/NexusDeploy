"use client";

import Link from "next/link";
import { LightPillars } from "@/components/ui/LightPillars";
import { useAuthStore } from "@/lib/store/authStore";
import {
  ArrowRight,
  GitBranch,
  Shield,
  Cpu,
  LayoutDashboard,
  Rocket,
  Terminal,
  Activity,
  Server,
  Database,
  Lock,
  Code,
  Workflow,
  GitCommit,
  Globe,
  Check,
  Sparkles,
  BarChart,
  PlayCircle,
  Loader2,
} from "lucide-react";

export default function HomePage() {
  const { isAuthenticated, accessToken, isLoading } = useAuthStore();

  // AuthProvider already handles isLoading at root level,
  // but we add this check here as an extra safety measure
  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  const isLoggedIn = isAuthenticated && accessToken;

  const stats = [
    { label: "Active Deployments", value: "10K+", icon: Rocket },
    { label: "Projects Deployed", value: "50K+", icon: Code },
    { label: "Builds Processed", value: "1M+", icon: Workflow },
    { label: "Uptime", value: "99.9%", icon: Activity },
  ];

  const features = [
    {
      title: "AI Error Analysis",
      description: "Intelligent analysis of build errors with actionable suggestions. Get detailed insights and fix recommendations powered by AI.",
      icon: Sparkles,
      color: "primary",
    },
    {
      title: "GitHub Integration",
      description: "Seamless connection with your repositories and automatic deployments. Push code and watch it deploy instantly.",
      icon: GitBranch,
      color: "accent-cyan",
    },
    {
      title: "Secure by Default",
      description: "End-to-end encryption for your secrets and environment variables. Enterprise-grade security built-in.",
      icon: Shield,
      color: "accent-emerald",
    },
    {
      title: "Auto Deploy",
      description: "Automated deployments on every push. Configure once and let the platform handle continuous delivery.",
      icon: Rocket,
      color: "primary",
    },
    {
      title: "Real-time Logs",
      description: "Stream build and deployment logs in real-time. Monitor your application's health with live updates.",
      icon: Terminal,
      color: "accent-cyan",
    },
    {
      title: "Resource Management",
      description: "Efficient container resource allocation. Scale resources based on your plan and application needs.",
      icon: Server,
      color: "accent-emerald",
    },
  ];

  const howItWorks = [
    {
      step: "1",
      title: "Connect Repository",
      description: "Link your GitHub repository with a single click. The platform automatically sets up webhooks for seamless integration.",
      icon: GitBranch,
    },
    {
      step: "2",
      title: "Auto Build",
      description: "Every push triggers an automated build. The system clones your code, runs tests, and builds Docker images automatically.",
      icon: Workflow,
    },
    {
      step: "3",
      title: "Deploy & Monitor",
      description: "Deployments happen automatically on successful builds. Monitor your application with real-time logs and metrics.",
      icon: Rocket,
    },
  ];

  const capabilities = [
    {
      title: "CI/CD Pipeline",
      description: "Complete continuous integration and deployment pipeline with automated testing and builds.",
      icon: GitCommit,
      color: "primary",
    },
    {
      title: "Container Management",
      description: "Docker-based container orchestration with automatic scaling and resource management.",
      icon: Server,
      color: "accent-cyan",
    },
    {
      title: "Environment Variables",
      description: "Secure environment variable management with encryption and per-project isolation.",
      icon: Database,
      color: "accent-emerald",
    },
    {
      title: "Webhooks",
      description: "Automated webhook configuration for GitHub events. Real-time notifications and triggers.",
      icon: Globe,
      color: "primary",
    },
    {
      title: "Monitoring",
      description: "Comprehensive monitoring with real-time logs, build status tracking, and deployment health checks.",
      icon: BarChart,
      color: "accent-cyan",
    },
    {
      title: "Security",
      description: "Built-in security features including encrypted secrets, access control, and secure deployments.",
      icon: Lock,
      color: "accent-emerald",
    },
  ];

  const planFeatures = {
    standard: [
      "3 Projects",
      "1 Concurrent Build",
      "512MB RAM per App",
      "1 CPU Core per App",
      "General AI Suggestions",
      "Community Support",
    ],
    premium: [
      "20 Projects",
      "5 Concurrent Builds",
      "2GB RAM per App",
      "2 CPU Cores per App",
      "Detailed AI Analysis & Error Fixes",
      "Custom Domain",
      "Email & Chat Support",
    ],
  };

  return (
    <div className="relative min-h-screen bg-background">
      {/* Fixed Background Container */}
      <div className="fixed inset-0 z-0">
        {/* Light Pillars Background */}
        <LightPillars />

        {/* Noise overlay */}
        <div className="absolute inset-0 bg-noise opacity-30 pointer-events-none" />
      </div>

      {/* Content */}
      <div className="relative z-10">
        {/* Header */}
        <header className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-3xl font-bold text-foreground">NexusDeploy</span>
            </div>
            {isLoggedIn ? null : (
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
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 pb-20">
          <div className="flex min-h-[calc(100vh-200px)] flex-col items-center justify-center text-center">
            {/* Heading */}
            <h1 className="mb-6 max-w-4xl text-5xl font-bold tracking-tight text-foreground sm:text-6xl lg:text-7xl animate-slide-up">
              Deploy with{" "}
              <span className="text-gradient">Intelligence</span>
            </h1>

            {/* Subheading */}
            <p className="mb-10 max-w-2xl text-lg text-white sm:text-xl animate-slide-up" style={{ animationDelay: "0.1s" }}>
              The next-generation PaaS platform with AI-powered error analysis. 
              Connect your GitHub, push your code, and let NexusDeploy handle the rest.
            </p>

            {/* CTA Buttons */}
            <div className="flex flex-col gap-4 sm:flex-row mb-16 animate-slide-up" style={{ animationDelay: "0.2s" }}>
              {isLoggedIn ? (
                <Link
                  href="/dashboard"
                  className="group inline-flex items-center justify-center gap-2 rounded-xl bg-primary px-8 py-4 text-base font-semibold text-white shadow-lg shadow-primary/30 transition-all duration-300 hover:bg-primary-600 hover:shadow-xl hover:shadow-primary/40 hover:-translate-y-0.5"
                >
                  <Rocket className="h-5 w-5" />
                  Enter Dashboard
                  <ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-1" />
                </Link>
              ) : (
                <Link
                  href="/login"
                  className="group inline-flex items-center justify-center gap-2 rounded-xl bg-primary px-8 py-4 text-base font-semibold text-white shadow-lg shadow-primary/30 transition-all duration-300 hover:bg-primary-600 hover:shadow-xl hover:shadow-primary/40 hover:-translate-y-0.5"
                >
                  <GitBranch className="h-5 w-5" />
                  Login with GitHub
                  <ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-1" />
                </Link>
              )}
              <Link
                href="https://github.com/Khoi1909/NexusDeploy"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-xl border border-surface-700 bg-surface-900/50 px-8 py-4 text-base font-semibold text-foreground backdrop-blur-sm transition-all duration-300 hover:border-surface-600 hover:bg-surface-800/50"
              >
                View on GitHub
              </Link>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 w-full max-w-4xl animate-slide-up" style={{ animationDelay: "0.3s" }}>
              {stats.map((stat, index) => (
                <div
                  key={index}
                  className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70"
                >
                  <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 mx-auto">
                    <stat.icon className="h-5 w-5 text-primary" />
                  </div>
                  <div className="text-2xl font-bold text-foreground mb-1">{stat.value}</div>
                  <div className="text-sm text-surface-400">{stat.label}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Features Showcase */}
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-foreground sm:text-4xl mb-4">
              Powerful Features
            </h2>
            <p className="text-lg text-surface-400 max-w-2xl mx-auto">
              Everything you need to deploy and manage your applications with confidence.
            </p>
          </div>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((feature, index) => {
              const Icon = feature.icon;
              const colorClasses = {
                primary: "bg-primary/10 text-primary",
                "accent-cyan": "bg-accent-cyan/10 text-accent-cyan",
                "accent-emerald": "bg-accent-emerald/10 text-accent-emerald",
              };
              return (
                <div
                  key={index}
                  className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70 hover:-translate-y-1"
                >
                  <div className={`mb-4 flex h-12 w-12 items-center justify-center rounded-lg ${colorClasses[feature.color as keyof typeof colorClasses]}`}>
                    <Icon className="h-6 w-6" />
                  </div>
                  <h3 className="mb-2 text-lg font-semibold text-foreground">{feature.title}</h3>
                  <p className="text-sm text-surface-400 leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              );
            })}
          </div>
        </section>

        {/* How It Works */}
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-foreground sm:text-4xl mb-4">
              How It Works
            </h2>
            <p className="text-lg text-surface-400 max-w-2xl mx-auto">
              Get started in minutes with our simple three-step process.
            </p>
          </div>
          <div className="grid grid-cols-1 gap-8 md:grid-cols-3 max-w-5xl mx-auto">
            {howItWorks.map((step, index) => {
              const Icon = step.icon;
              return (
                <div key={index} className="relative">
                  {index < howItWorks.length - 1 && (
                    <div className="hidden md:block absolute top-12 left-full w-full h-0.5 bg-gradient-to-r from-primary/50 to-transparent transform translate-x-4" style={{ width: "calc(100% - 2rem)" }} />
                  )}
                  <div className="text-center">
                    <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 border-2 border-primary/30 mx-auto">
                      <Icon className="h-8 w-8 text-primary" />
                    </div>
                    <div className="mb-2 inline-flex h-8 w-8 items-center justify-center rounded-full bg-primary text-sm font-bold text-white">
                      {step.step}
                    </div>
                    <h3 className="mb-3 text-xl font-semibold text-foreground">{step.title}</h3>
                    <p className="text-sm text-surface-400 leading-relaxed">
                      {step.description}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        {/* Platform Capabilities */}
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-foreground sm:text-4xl mb-4">
              Platform Capabilities
            </h2>
            <p className="text-lg text-surface-400 max-w-2xl mx-auto">
              Built for modern development workflows with enterprise-grade reliability.
            </p>
          </div>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {capabilities.map((capability, index) => {
              const Icon = capability.icon;
              const colorClasses = {
                primary: "bg-primary/10 text-primary",
                "accent-cyan": "bg-accent-cyan/10 text-accent-cyan",
                "accent-emerald": "bg-accent-emerald/10 text-accent-emerald",
              };
              return (
                <div
                  key={index}
                  className="rounded-xl border border-surface-800 bg-surface-900/50 p-6 backdrop-blur-sm transition-all duration-300 hover:border-surface-700 hover:bg-surface-900/70"
                >
                  <div className={`mb-4 flex h-12 w-12 items-center justify-center rounded-lg ${colorClasses[capability.color as keyof typeof colorClasses]}`}>
                    <Icon className="h-6 w-6" />
                  </div>
                  <h3 className="mb-2 text-lg font-semibold text-foreground">{capability.title}</h3>
                  <p className="text-sm text-surface-400 leading-relaxed">
                    {capability.description}
                  </p>
                </div>
              );
            })}
          </div>
        </section>

        {/* Plans Preview */}
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-foreground sm:text-4xl mb-4">
              Choose Your Plan
            </h2>
            <p className="text-lg text-surface-400 max-w-2xl mx-auto">
              Start free and scale as you grow. All plans include core features.
            </p>
          </div>
          <div className="grid grid-cols-1 gap-8 md:grid-cols-2 max-w-4xl mx-auto">
            {/* Standard Plan */}
            <div className="rounded-xl border border-surface-800 bg-surface-900/50 p-8 backdrop-blur-sm">
              <div className="mb-6">
                <h3 className="text-2xl font-bold text-foreground mb-2">Standard</h3>
                <p className="text-surface-400">Perfect for personal projects and small teams</p>
              </div>
              <ul className="space-y-3 mb-8">
                {planFeatures.standard.map((feature, index) => (
                  <li key={index} className="flex items-start gap-3">
                    <Check className="h-5 w-5 text-accent-emerald shrink-0 mt-0.5" />
                    <span className="text-surface-300">{feature}</span>
                  </li>
                ))}
              </ul>
              <Link
                href="/login"
                className="block w-full text-center rounded-lg bg-surface-800 px-6 py-3 text-sm font-medium text-foreground transition-colors hover:bg-surface-700"
              >
                Get Started
              </Link>
            </div>

            {/* Premium Plan */}
            <div className="rounded-xl border-2 border-primary/50 bg-surface-900/50 p-8 backdrop-blur-sm relative">
              <div className="absolute -top-4 left-1/2 transform -translate-x-1/2">
                <span className="inline-flex items-center gap-1 rounded-full bg-primary px-3 py-1 text-xs font-medium text-white">
                  <Sparkles className="h-3 w-3" />
                  Popular
                </span>
              </div>
              <div className="mb-6">
                <h3 className="text-2xl font-bold text-foreground mb-2">Premium</h3>
                <p className="text-surface-400">For teams and applications that need more power</p>
              </div>
              <ul className="space-y-3 mb-8">
                {planFeatures.premium.map((feature, index) => (
                  <li key={index} className="flex items-start gap-3">
                    <Check className="h-5 w-5 text-accent-emerald shrink-0 mt-0.5" />
                    <span className="text-surface-300">{feature}</span>
                  </li>
                ))}
              </ul>
              <Link
                href="/login"
                className="block w-full text-center rounded-lg bg-primary px-6 py-3 text-sm font-medium text-white transition-colors hover:bg-primary-600"
              >
                Upgrade to Premium
              </Link>
            </div>
          </div>
        </section>

        {/* Final CTA */}
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-20">
          <div className="rounded-2xl border border-surface-800 bg-surface-900/50 p-12 text-center backdrop-blur-sm">
            <h2 className="text-3xl font-bold text-foreground sm:text-4xl mb-4">
              Ready to Deploy?
            </h2>
            <p className="text-lg text-surface-400 mb-8 max-w-2xl mx-auto">
              Join thousands of developers who are already deploying with NexusDeploy.
            </p>
            <Link
              href="/login"
              className="inline-flex items-center justify-center gap-2 rounded-xl bg-primary px-8 py-4 text-base font-semibold text-white shadow-lg shadow-primary/30 transition-all duration-300 hover:bg-primary-600 hover:shadow-xl hover:shadow-primary/40"
            >
              <PlayCircle className="h-5 w-5" />
              Get Started Free
              <ArrowRight className="h-5 w-5" />
            </Link>
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-surface-800 mt-20">
          <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
            <div className="grid grid-cols-1 gap-8 md:grid-cols-4">
              {/* Brand */}
              <div className="md:col-span-1">
                <div className="mb-4">
                  <span className="text-xl font-bold text-foreground">NexusDeploy</span>
                </div>
                <p className="text-sm text-surface-500">
                  The next-generation PaaS platform for modern developers.
                </p>
              </div>

              {/* Resources */}
              <div>
                <h4 className="text-sm font-semibold text-foreground mb-4">Resources</h4>
                <ul className="space-y-3">
                  <li>
                    <Link href="/help" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                      Documentation
                    </Link>
                  </li>
                  <li>
                    <Link href="/help" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                      Help Center
                    </Link>
                  </li>
                  <li>
                    <a
                      href="https://github.com/Khoi1909/NexusDeploy"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-surface-500 transition-colors hover:text-foreground"
                    >
                      GitHub
                    </a>
                  </li>
                </ul>
              </div>

              {/* Company */}
              <div>
                <h4 className="text-sm font-semibold text-foreground mb-4">Platform</h4>
                <ul className="space-y-3">
                  <li>
                    <Link href="/dashboard" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                      Dashboard
                    </Link>
                  </li>
                  <li>
                    <Link href="/projects/new" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                      New Project
                    </Link>
                  </li>
                  <li>
                    <Link href="/settings" className="text-sm text-surface-500 transition-colors hover:text-foreground">
                      Settings
                    </Link>
                  </li>
                </ul>
              </div>

              {/* Legal */}
              <div>
                <h4 className="text-sm font-semibold text-foreground mb-4">Legal</h4>
                <ul className="space-y-3">
                  <li>
                    <span className="text-sm text-surface-500">Privacy Policy</span>
                  </li>
                  <li>
                    <span className="text-sm text-surface-500">Terms of Service</span>
                  </li>
                  <li>
                    <span className="text-sm text-surface-500">
                      © 2024 NexusDeploy
                    </span>
                  </li>
                </ul>
              </div>
            </div>
            <div className="mt-8 pt-8 border-t border-surface-800">
              <p className="text-center text-sm text-surface-500">
                Built for developers, by developers. Open source and ready for production.
              </p>
            </div>
          </div>
        </footer>
      </div>
    </div>
  );
}
