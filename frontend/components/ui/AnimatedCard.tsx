"use client";

import { HTMLAttributes, forwardRef } from "react";
import { cn } from "@/lib/utils/cn";

type StatusType = "running" | "stopped" | "building" | "error" | "pending";

interface AnimatedCardProps extends HTMLAttributes<HTMLDivElement> {
  status?: StatusType;
  glowOnHover?: boolean;
}

const statusConfig: Record<StatusType, { border: string; glow: string; indicator: string }> = {
  running: {
    border: "border-accent-emerald/30 hover:border-accent-emerald/60",
    glow: "hover:shadow-accent-emerald/20",
    indicator: "bg-accent-emerald",
  },
  stopped: {
    border: "border-surface-600 hover:border-surface-500",
    glow: "hover:shadow-surface-600/20",
    indicator: "bg-surface-500",
  },
  building: {
    border: "border-accent-amber/30 hover:border-accent-amber/60",
    glow: "hover:shadow-accent-amber/20",
    indicator: "bg-accent-amber animate-pulse",
  },
  error: {
    border: "border-accent-rose/30 hover:border-accent-rose/60",
    glow: "hover:shadow-accent-rose/20",
    indicator: "bg-accent-rose",
  },
  pending: {
    border: "border-surface-700 hover:border-primary/40",
    glow: "hover:shadow-primary/10",
    indicator: "bg-surface-600",
  },
};

export const AnimatedCard = forwardRef<HTMLDivElement, AnimatedCardProps>(
  ({ className, status = "pending", glowOnHover = true, children, ...props }, ref) => {
    const config = statusConfig[status];

    return (
      <div
        ref={ref}
        className={cn(
          "group relative rounded-xl bg-surface-900 p-6",
          "border transition-all duration-300 ease-out",
          "hover:-translate-y-1 hover:shadow-xl",
          config.border,
          glowOnHover && config.glow,
          className
        )}
        {...props}
      >
        {/* Status indicator */}
        <div className="absolute top-4 right-4 flex items-center gap-2">
          <div className={cn("h-2 w-2 rounded-full", config.indicator)} />
        </div>

        {/* Gradient overlay on hover */}
        <div
          className={cn(
            "pointer-events-none absolute inset-0 rounded-xl opacity-0 transition-opacity duration-300",
            "bg-gradient-to-br from-primary/5 via-transparent to-accent-cyan/5",
            "group-hover:opacity-100"
          )}
        />

        {/* Content */}
        <div className="relative z-10">{children}</div>
      </div>
    );
  }
);

AnimatedCard.displayName = "AnimatedCard";

