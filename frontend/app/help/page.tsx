"use client";

import { useState } from "react";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import {
  HelpCircle,
  BookOpen,
  MessageCircle,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Github,
  Mail,
  CheckCircle,
} from "lucide-react";

interface FAQItem {
  question: string;
  answer: string;
}

const faqItems: FAQItem[] = [
  {
    question: "How do I create a new project?",
    answer: "Navigate to 'New Project' from the sidebar or dashboard. Select a repository from your GitHub, choose a suitable preset (Node.js, Go, etc.), and click 'Create Project'. The system will automatically install a webhook and be ready for your first push.",
  },
  {
    question: "How does the CI/CD pipeline work?",
    answer: "When you push code to GitHub, the webhook will automatically trigger a build. The system will clone the code, run build/test in a Docker container, then build a Docker image and deploy the application. You can track progress through real-time logs.",
  },
  {
    question: "How do I add environment variables (Secrets)?",
    answer: "Go to the project detail page, select the 'Secrets' tab. You can add, edit, or delete secrets at any time. Secrets will be encrypted and automatically injected into containers during build and deployment.",
  },
  {
    question: "What is the 'Tell me why' feature?",
    answer: "When a build fails, you can click the 'Tell me why' button for AI to analyze error logs and provide suggestions. Premium plan users will receive more detailed analysis compared to Standard plan users.",
  },
  {
    question: "How do I stop a deployment?",
    answer: "Go to the project detail page or dashboard, find the running deployment and click the 'Stop' button. The container will be stopped and deleted, but you can deploy again at any time.",
  },
  {
    question: "How are domains assigned?",
    answer: "Each project is automatically assigned a subdomain based on the project ID (e.g., {project-id}.localhost). With the Premium plan, you can use a custom domain.",
  },
  {
    question: "Are there resource limits?",
    answer: "Yes, limits depend on your subscription plan. Standard plan: 512MB RAM, 1 CPU core. Premium plan: 2GB RAM, 2 CPU cores. Limits are automatically applied to each container.",
  },
  {
    question: "How do I view logs for a running application?",
    answer: "Go to the project detail page, select the 'Builds' tab to view build logs, or the 'Overview' tab to view deployment logs. Logs are streamed in real-time via WebSocket.",
  },
];

export default function HelpPage() {
  const { user } = useAuthStore();
  const [expandedFAQ, setExpandedFAQ] = useState<number | null>(null);
  const [activeSection, setActiveSection] = useState<string>("getting-started");

  const toggleFAQ = (index: number) => {
    setExpandedFAQ(expandedFAQ === index ? null : index);
  };

  const sections = [
    { id: "getting-started", label: "Getting Started" },
    { id: "creating-project", label: "Creating a Project" },
    { id: "cicd", label: "CI/CD Pipeline" },
    { id: "troubleshooting", label: "Troubleshooting" },
    { id: "faq", label: "FAQ" },
  ];

  return (
    <div className="relative min-h-screen bg-background">
      <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
      <Navbar />

      <div className="flex">
        <Sidebar />

        <main className="flex-1 lg:ml-64 pt-16">
          <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
            {/* Header */}
            <div className="mb-8">
              <h1 className="text-2xl font-bold text-foreground sm:text-3xl">Help & Documentation</h1>
              <p className="mt-1 text-surface-400">
                Learn how to use NexusDeploy and get support
              </p>
            </div>

            <div className="grid gap-6 lg:grid-cols-4">
              {/* Sidebar Navigation */}
              <div className="lg:col-span-1">
                <Card variant="elevated" className="sticky top-24">
                  <nav className="space-y-1">
                    {sections.map((section) => (
                      <button
                        key={section.id}
                        onClick={() => setActiveSection(section.id)}
                        className={`w-full rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors ${
                          activeSection === section.id
                            ? "bg-primary/10 text-primary"
                            : "text-surface-400 hover:bg-surface-800 hover:text-foreground"
                        }`}
                      >
                        {section.label}
                      </button>
                    ))}
                  </nav>
                </Card>
              </div>

              {/* Main Content */}
              <div className="lg:col-span-3 space-y-6">
                {/* Getting Started */}
                {activeSection === "getting-started" && (
                  <Card variant="elevated">
                    <h2 className="mb-4 text-xl font-semibold text-foreground">Getting Started</h2>
                    <div className="prose prose-invert max-w-none space-y-4 text-sm text-surface-300">
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Welcome to NexusDeploy</h3>
                        <p>
                          NexusDeploy is a PaaS (Platform-as-a-Service) platform that helps you deploy web applications automatically from GitHub repositories.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Quick Start</h3>
                        <ol className="ml-6 list-decimal space-y-2">
                          <li>Log in with your GitHub account</li>
                          <li>Create a new project from a GitHub repository</li>
                          <li>Push code to the repository - the system will automatically build and deploy</li>
                          <li>Track progress through the dashboard and real-time logs</li>
                        </ol>
                      </div>
                    </div>
                  </Card>
                )}

                {/* Creating a Project */}
                {activeSection === "creating-project" && (
                  <Card variant="elevated">
                    <h2 className="mb-4 text-xl font-semibold text-foreground">Creating a Project</h2>
                    <div className="space-y-4 text-sm text-surface-300">
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Step 1: Choose Repository</h3>
                        <p>Select a repository from your GitHub repositories list. The repository must be a web application.</p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Step 2: Select Preset</h3>
                        <p>
                          Choose a preset that matches your stack (Node.js, Go, Python, etc.). The preset will provide default build/start commands.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Step 3: Configure (Optional)</h3>
                        <p>
                          You can customize the build command and start command if needed. Add environment variables (secrets) if your application requires them.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Step 4: Create</h3>
                        <p>
                          Click "Create Project". The system will automatically install a webhook in the GitHub repository and be ready for your first push.
                        </p>
                      </div>
                    </div>
                  </Card>
                )}

                {/* CI/CD Pipeline */}
                {activeSection === "cicd" && (
                  <Card variant="elevated">
                    <h2 className="mb-4 text-xl font-semibold text-foreground">How CI/CD Works</h2>
                    <div className="space-y-4 text-sm text-surface-300">
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Automatic Trigger</h3>
                        <p>
                          When you push code to GitHub (git push), the webhook will automatically send an event to NexusDeploy and trigger the CI/CD process.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">CI Phase (Build & Test)</h3>
                        <p>
                          The system will clone the code, run build and test commands in an isolated Docker container. You can track logs in real-time.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">CD Phase (Deploy)</h3>
                        <p>
                          If CI succeeds, the system will build a Docker image, push it to the registry, and automatically deploy the application. The application will be assigned an automatic domain with HTTPS.
                        </p>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Manual Actions</h3>
                        <p>
                          You can also trigger builds manually from the UI, or stop/restart deployments at any time.
                        </p>
                      </div>
                    </div>
                  </Card>
                )}

                {/* Troubleshooting */}
                {activeSection === "troubleshooting" && (
                  <Card variant="elevated">
                    <h2 className="mb-4 text-xl font-semibold text-foreground">Troubleshooting</h2>
                    <div className="space-y-4 text-sm text-surface-300">
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Build Failed</h3>
                        <p className="mb-2">If the build fails:</p>
                        <ul className="ml-6 list-disc space-y-1">
                          <li>Check build logs to see detailed errors</li>
                          <li>Use the "Tell me why" feature for AI error analysis</li>
                          <li>Ensure build command and start command are correct</li>
                          <li>Check dependencies in package.json/go.mod/etc.</li>
                        </ul>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Deployment Failed</h3>
                        <p className="mb-2">If the deployment fails:</p>
                        <ul className="ml-6 list-disc space-y-1">
                          <li>Check if the start command is correct</li>
                          <li>Check port configuration</li>
                          <li>Check if environment variables are complete</li>
                          <li>View deployment logs for details</li>
                        </ul>
                      </div>
                      <div>
                        <h3 className="mb-2 text-lg font-medium text-foreground">Application Not Accessible</h3>
                        <p className="mb-2">If the application is not accessible:</p>
                        <ul className="ml-6 list-disc space-y-1">
                          <li>Check deployment status - is the container running</li>
                          <li>Check the public URL from the deployment status card</li>
                          <li>Check logs to see if the application has errors on startup</li>
                          <li>Try restarting the deployment</li>
                        </ul>
                      </div>
                    </div>
                  </Card>
                )}

                {/* FAQ */}
                {activeSection === "faq" && (
                  <Card variant="elevated">
                    <h2 className="mb-4 text-xl font-semibold text-foreground">Frequently Asked Questions</h2>
                    <div className="space-y-3">
                      {faqItems.map((item, index) => (
                        <div
                          key={index}
                          className="rounded-lg border border-surface-800 bg-surface-900/50 overflow-hidden"
                        >
                          <button
                            onClick={() => toggleFAQ(index)}
                            className="flex w-full items-center justify-between p-4 text-left transition-colors hover:bg-surface-800/50"
                          >
                            <span className="font-medium text-foreground">{item.question}</span>
                            {expandedFAQ === index ? (
                              <ChevronUp className="h-5 w-5 text-surface-400" />
                            ) : (
                              <ChevronDown className="h-5 w-5 text-surface-400" />
                            )}
                          </button>
                          {expandedFAQ === index && (
                            <div className="border-t border-surface-800 p-4 text-sm text-surface-300">
                              {item.answer}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </Card>
                )}

                {/* Support */}
                <Card variant="elevated">
                  <h2 className="mb-4 text-xl font-semibold text-foreground">Get Support</h2>
                  <div className="space-y-4">
                    <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-4">
                      <div className="flex items-start gap-3">
                        <Github className="h-5 w-5 text-surface-400 mt-0.5" />
                        <div>
                          <h3 className="font-medium text-foreground mb-1">GitHub Repository</h3>
                          <p className="text-sm text-surface-300 mb-2">
                            Report issues, request features, or contribute to the project.
                          </p>
                          <a
                            href="https://github.com/Khoi1909/NexusDeploy"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-2 text-sm text-primary hover:underline"
                          >
                            View on GitHub
                            <ExternalLink className="h-3.5 w-3.5" />
                          </a>
                        </div>
                      </div>
                    </div>

                    {user?.plan === "premium" ? (
                      <div className="rounded-lg border border-primary/30 bg-primary/10 p-4">
                        <div className="flex items-start gap-3">
                          <Mail className="h-5 w-5 text-primary mt-0.5" />
                          <div>
                            <h3 className="font-medium text-foreground mb-1">Premium Support</h3>
                            <p className="text-sm text-surface-300">
                              As a Premium user, you have access to email and chat support. Contact us for priority assistance.
                            </p>
                          </div>
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-4">
                        <div className="flex items-start gap-3">
                          <MessageCircle className="h-5 w-5 text-surface-400 mt-0.5" />
                          <div>
                            <h3 className="font-medium text-foreground mb-1">Community Support</h3>
                            <p className="text-sm text-surface-300">
                              Join our community discussions, check GitHub issues, or upgrade to Premium for priority support.
                            </p>
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </Card>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}
