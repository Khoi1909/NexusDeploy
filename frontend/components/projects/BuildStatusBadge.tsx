import { cn } from "@/lib/utils/cn";

export type BuildStatus =
  | "pending"
  | "running"
  | "failed"
  | "building_image"
  | "pushing_image"
  | "deploying"
  | "success"
  | "deploy_failed";

interface BuildStatusBadgeProps {
  status: string;
  className?: string;
}

export function BuildStatusBadge({ status, className }: BuildStatusBadgeProps) {
  const statusConfig: Record<
    string,
    { label: string; className: string }
  > = {
    pending: {
      label: "Pending",
      className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20",
    },
    running: {
      label: "Running",
      className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
    },
    building_image: {
      label: "Building Image",
      className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
    },
    pushing_image: {
      label: "Pushing Image",
      className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
    },
    deploying: {
      label: "Deploying",
      className: "bg-purple-500/10 text-purple-500 border-purple-500/20",
    },
    success: {
      label: "Success",
      className: "bg-green-500/10 text-green-500 border-green-500/20",
    },
    failed: {
      label: "Failed",
      className: "bg-red-500/10 text-red-500 border-red-500/20",
    },
    deploy_failed: {
      label: "Deploy Failed",
      className: "bg-red-500/10 text-red-500 border-red-500/20",
    },
  };

  const config = statusConfig[status.toLowerCase()] || {
    label: status,
    className: "bg-surface-800 text-surface-400 border-surface-700",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium",
        config.className,
        className
      )}
    >
      {config.label}
    </span>
  );
}

