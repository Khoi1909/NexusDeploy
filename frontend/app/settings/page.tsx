"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { DotGrid } from "@/components/ui/DotGrid";
import { Navbar } from "@/components/layout/Navbar";
import { Sidebar } from "@/components/layout/Sidebar";
import { Card } from "@/components/common/Card";
import { useAuthStore } from "@/lib/store/authStore";
import { useRouter } from "next/navigation";
import {
  User,
  Settings as SettingsIcon,
  Crown,
  Check,
  X,
  Mail,
  Calendar,
  Github,
  Loader2,
  AlertCircle,
} from "lucide-react";
import { authApi } from "@/lib/api/auth";

interface PlanLimits {
  maxProjects: number;
  maxConcurrentBuilds: number;
  maxRamMb: number;
  maxCpuCores: number;
  aiAnalysis: string;
  customDomain: boolean;
  support: string;
}

const planLimits: Record<string, PlanLimits> = {
  standard: {
    maxProjects: 3,
    maxConcurrentBuilds: 1,
    maxRamMb: 512,
    maxCpuCores: 1,
    aiAnalysis: "General suggestions",
    customDomain: false,
    support: "Community",
  },
  premium: {
    maxProjects: 20,
    maxConcurrentBuilds: 5,
    maxRamMb: 2048,
    maxCpuCores: 2,
    aiAnalysis: "Detailed suggestions & Error fixes",
    customDomain: true,
    support: "Email & Chat",
  },
};

export default function SettingsPage() {
  const router = useRouter();
  const { user, accessToken, isLoading: authLoading, isAuthenticated, logout, setUser } = useAuthStore();
  const [isLoading, setIsLoading] = useState(true);
  const [isUpdatingPlan, setIsUpdatingPlan] = useState(false);
  const [planUpdateError, setPlanUpdateError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) {
      return;
    }

    if (!isAuthenticated && !accessToken) {
      setIsLoading(false);
      router.push("/");
      return;
    }

    setIsLoading(false);
  }, [accessToken, authLoading, isAuthenticated, router]);

  if (isLoading || !user) {
    return (
      <div className="relative min-h-screen bg-background">
        <DotGrid dotColor="#27272a" spacing={28} fadeEdges />
        <Navbar />
        <div className="flex">
          <Sidebar />
          <main className="flex-1 lg:ml-64 pt-16">
            <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
              <div className="flex items-center justify-center py-12">
                <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
              </div>
            </div>
          </main>
        </div>
      </div>
    );
  }

  const currentPlan = user.plan || "standard";
  const limits = planLimits[currentPlan];

  const handlePlanChange = async (newPlan: "standard" | "premium") => {
    if (!accessToken) {
      setPlanUpdateError("Not authenticated");
      return;
    }

    if (currentPlan === newPlan) {
      return;
    }

    const confirmMessage = newPlan === "premium"
      ? "Do you want to upgrade to Premium plan?"
      : "Are you sure you want to downgrade to Standard plan? Some features may be limited.";

    if (!confirm(confirmMessage)) {
      return;
    }

    setIsUpdatingPlan(true);
    setPlanUpdateError(null);

    try {
      const response = await authApi.updatePlan(accessToken, newPlan);
      if (response.success) {
        // Update user in store
        setUser({ ...user, plan: newPlan });
        alert(response.message || `Plan đã được cập nhật thành ${newPlan}`);
      } else {
        setPlanUpdateError("Failed to update plan");
      }
    } catch (err: any) {
      console.error("Failed to update plan:", err);
      const errorMessage = err.message || err.error || "Failed to update plan";
      setPlanUpdateError(errorMessage);
    } finally {
      setIsUpdatingPlan(false);
    }
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
            <div className="mb-8">
              <h1 className="text-2xl font-bold text-foreground sm:text-3xl">Settings</h1>
              <p className="mt-1 text-surface-400">
                Manage your account and subscription
              </p>
            </div>

            <div className="grid gap-6 lg:grid-cols-3">
              {/* User Profile */}
              <div className="lg:col-span-2 space-y-6">
                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">Profile Information</h3>
                  <div className="space-y-4">
                    <div className="flex items-center gap-4">
                      {user.avatar_url ? (
                        <img
                          src={user.avatar_url}
                          alt={user.username}
                          className="h-16 w-16 rounded-full border-2 border-surface-700"
                        />
                      ) : (
                        <div className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-surface-700 bg-surface-800">
                          <User className="h-8 w-8 text-surface-400" />
                        </div>
                      )}
                      <div>
                        <h4 className="text-lg font-semibold text-foreground">{user.username}</h4>
                        <p className="text-sm text-surface-400">GitHub Account</p>
                      </div>
                    </div>

                    <div className="space-y-3 border-t border-surface-800 pt-4">
                      <div>
                        <label className="text-sm text-surface-400">Email</label>
                        <p className="mt-1 text-foreground">{user.email || "Not provided"}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">GitHub ID</label>
                        <p className="mt-1 font-mono text-sm text-foreground">{user.id}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">Member since</label>
                        <p className="mt-1 text-foreground">
                          {/* User created_at not in store, can't display */}
                          Account linked via GitHub
                        </p>
                      </div>
                    </div>

                    <div className="rounded-lg bg-surface-900/50 border border-surface-800 p-3">
                      <p className="text-xs text-surface-400">
                        Profile information is synced from your GitHub account and cannot be edited here.
                      </p>
                    </div>
                  </div>
                </Card>

                {/* Plan Information */}
                <Card variant="elevated">
                  <div className="mb-4 flex items-center justify-between">
                    <h3 className="text-lg font-semibold text-foreground">Plan Information</h3>
                    <span
                      className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${
                        currentPlan === "premium"
                          ? "bg-yellow-500/10 text-yellow-500"
                          : "bg-surface-800 text-surface-400"
                      }`}
                    >
                      {currentPlan === "premium" && <Crown className="h-3.5 w-3.5" />}
                      {currentPlan === "premium" ? "Premium" : "Standard"}
                    </span>
                  </div>

                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm text-surface-400">Max Projects</label>
                        <p className="mt-1 text-foreground">{limits.maxProjects}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">Concurrent Builds</label>
                        <p className="mt-1 text-foreground">{limits.maxConcurrentBuilds}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">Max RAM per App</label>
                        <p className="mt-1 text-foreground">{limits.maxRamMb} MB</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">Max CPU per App</label>
                        <p className="mt-1 text-foreground">{limits.maxCpuCores} Core{limits.maxCpuCores > 1 ? "s" : ""}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">AI Analysis</label>
                        <p className="mt-1 text-foreground">{limits.aiAnalysis}</p>
                      </div>
                      <div>
                        <label className="text-sm text-surface-400">Custom Domain</label>
                        <p className="mt-1 text-foreground">
                          {limits.customDomain ? (
                            <span className="text-accent-emerald">Available</span>
                          ) : (
                            <span className="text-surface-400">Not available</span>
                          )}
                        </p>
                      </div>
                    </div>

                    <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-4">
                      <p className="text-sm text-surface-400 mb-2">Support</p>
                      <p className="text-foreground">{limits.support}</p>
                    </div>

                    {/* Plan Change Section */}
                    <div className="rounded-lg border border-surface-800 bg-surface-900/50 p-4">
                      {currentPlan === "standard" ? (
                        <>
                          <p className="text-sm text-foreground mb-2 font-medium">
                            Upgrade to Premium for more features
                          </p>
                          <p className="text-xs text-surface-400 mb-3">
                            Get more projects, concurrent builds, resources, and priority support.
                          </p>
                          {planUpdateError && (
                            <div className="mb-3 flex items-center gap-2 rounded-lg border border-accent-rose/30 bg-accent-rose/10 p-2 text-sm text-accent-rose">
                              <AlertCircle className="h-4 w-4" />
                              <span>{planUpdateError}</span>
                            </div>
                          )}
                          <button
                            onClick={() => handlePlanChange("premium")}
                            disabled={isUpdatingPlan}
                            className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {isUpdatingPlan ? (
                              <>
                                <Loader2 className="h-4 w-4 animate-spin" />
                                Upgrading...
                              </>
                            ) : (
                              <>
                                <Crown className="h-4 w-4" />
                                Upgrade to Premium
                              </>
                            )}
                          </button>
                        </>
                      ) : (
                        <>
                          <p className="text-sm text-foreground mb-2 font-medium">
                            Downgrade to Standard
                          </p>
                          <p className="text-xs text-surface-400 mb-3">
                            You will lose some features such as custom domain, priority support, and higher limits.
                          </p>
                          {planUpdateError && (
                            <div className="mb-3 flex items-center gap-2 rounded-lg border border-accent-rose/30 bg-accent-rose/10 p-2 text-sm text-accent-rose">
                              <AlertCircle className="h-4 w-4" />
                              <span>{planUpdateError}</span>
                            </div>
                          )}
                          <button
                            onClick={() => handlePlanChange("standard")}
                            disabled={isUpdatingPlan}
                            className="inline-flex items-center gap-2 rounded-lg border border-surface-700 bg-surface-800 px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surface-700 disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {isUpdatingPlan ? (
                              <>
                                <Loader2 className="h-4 w-4 animate-spin" />
                                Downgrading...
                              </>
                            ) : (
                              "Downgrade to Standard"
                            )}
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                </Card>
              </div>

              {/* Plan Comparison */}
              <div className="lg:col-span-1">
                <Card variant="elevated">
                  <h3 className="mb-4 text-lg font-semibold text-foreground">Plan Comparison</h3>
                  <div className="space-y-4">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-surface-800">
                          <th className="pb-2 text-left text-xs font-medium text-surface-400">Feature</th>
                          <th className="pb-2 text-center text-xs font-medium text-surface-400">Standard</th>
                          <th className="pb-2 text-center text-xs font-medium text-surface-400">Premium</th>
                        </tr>
                      </thead>
                      <tbody className="text-foreground">
                        <tr className="border-b border-surface-800">
                          <td className="py-3 text-xs">Max Projects</td>
                          <td className="py-3 text-center text-xs">{planLimits.standard.maxProjects}</td>
                          <td className="py-3 text-center text-xs">{planLimits.premium.maxProjects}</td>
                        </tr>
                        <tr className="border-b border-surface-800">
                          <td className="py-3 text-xs">Concurrent Builds</td>
                          <td className="py-3 text-center text-xs">{planLimits.standard.maxConcurrentBuilds}</td>
                          <td className="py-3 text-center text-xs">{planLimits.premium.maxConcurrentBuilds}</td>
                        </tr>
                        <tr className="border-b border-surface-800">
                          <td className="py-3 text-xs">RAM per App</td>
                          <td className="py-3 text-center text-xs">{planLimits.standard.maxRamMb} MB</td>
                          <td className="py-3 text-center text-xs">{planLimits.premium.maxRamMb} MB</td>
                        </tr>
                        <tr className="border-b border-surface-800">
                          <td className="py-3 text-xs">CPU per App</td>
                          <td className="py-3 text-center text-xs">{planLimits.standard.maxCpuCores}</td>
                          <td className="py-3 text-center text-xs">{planLimits.premium.maxCpuCores}</td>
                        </tr>
                        <tr className="border-b border-surface-800">
                          <td className="py-3 text-xs">Custom Domain</td>
                          <td className="py-3 text-center">
                            {planLimits.standard.customDomain ? (
                              <Check className="mx-auto h-4 w-4 text-accent-emerald" />
                            ) : (
                              <X className="mx-auto h-4 w-4 text-surface-600" />
                            )}
                          </td>
                          <td className="py-3 text-center">
                            {planLimits.premium.customDomain ? (
                              <Check className="mx-auto h-4 w-4 text-accent-emerald" />
                            ) : (
                              <X className="mx-auto h-4 w-4 text-surface-600" />
                            )}
                          </td>
                        </tr>
                        <tr>
                          <td className="py-3 text-xs">Support</td>
                          <td className="py-3 text-center text-xs">{planLimits.standard.support}</td>
                          <td className="py-3 text-center text-xs">{planLimits.premium.support}</td>
                        </tr>
                      </tbody>
                    </table>
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
