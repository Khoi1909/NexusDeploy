"use client";

import { Build } from "@/lib/api/builds";
import { BuildStatusBadge } from "./BuildStatusBadge";
import { Card } from "@/components/common/Card";
import { cn } from "@/lib/utils/cn";
import { GitBranch, Clock, CheckCircle2, XCircle } from "lucide-react";

interface BuildCardProps {
  build: Build;
  onClick?: () => void;
  isExpanded?: boolean;
}

export function BuildCard({ build, onClick, isExpanded }: BuildCardProps) {
  const shortSha = build.commit_sha.substring(0, 7);
  const duration = build.started_at && build.finished_at
    ? Math.round(
        (new Date(build.finished_at).getTime() -
          new Date(build.started_at).getTime()) /
          1000
      )
    : null;

  const isSuccess = build.status === "success";
  const isFailed =
    build.status === "failed" || build.status === "deploy_failed";
  const isRunning =
    build.status === "running" ||
    build.status === "building_image" ||
    build.status === "pushing_image" ||
    build.status === "deploying";

  return (
    <Card
      className={cn(
        "cursor-pointer transition-all duration-200 hover:border-surface-600",
        isExpanded && "border-primary/50 bg-surface-900/50",
        isSuccess && "hover:border-green-500/30",
        isFailed && "hover:border-red-500/30"
      )}
      onClick={onClick}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 space-y-2">
          <div className="flex items-center gap-3">
            <BuildStatusBadge status={build.status} />
            <div className="flex items-center gap-2 text-sm text-surface-400">
              <GitBranch className="h-3.5 w-3.5" />
              <span className="font-mono">{shortSha}</span>
            </div>
          </div>

          <div className="flex items-center gap-4 text-xs text-surface-500">
            {build.started_at && (
              <div className="flex items-center gap-1.5">
                <Clock className="h-3 w-3" />
                <span>
                  {new Date(build.started_at).toLocaleString("vi-VN", {
                    day: "2-digit",
                    month: "2-digit",
                    year: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </div>
            )}
            {duration !== null && (
              <span className="text-surface-600">
                {duration}s
              </span>
            )}
          </div>
        </div>

        <div className="flex items-center">
          {isSuccess && (
            <CheckCircle2 className="h-5 w-5 text-green-500" />
          )}
          {isFailed && <XCircle className="h-5 w-5 text-red-500" />}
          {isRunning && (
            <div className="h-5 w-5 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
          )}
        </div>
      </div>
    </Card>
  );
}

