"use client";

import { HTMLAttributes, forwardRef } from "react";
import { cn } from "@/lib/utils/cn";

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "glass" | "elevated";
  hover?: boolean;
}

export const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant = "default", hover = false, children, ...props }, ref) => {
    const variants = {
      default: "bg-surface-900 border border-surface-800",
      glass: "glass",
      elevated: "bg-surface-900 border border-surface-800 shadow-xl shadow-black/20",
    };

    return (
      <div
        ref={ref}
        className={cn(
          "rounded-xl p-6",
          variants[variant],
          hover && "transition-all duration-300 hover:border-surface-600 hover:shadow-lg hover:-translate-y-0.5",
          className
        )}
        {...props}
      >
        {children}
      </div>
    );
  }
);

Card.displayName = "Card";

