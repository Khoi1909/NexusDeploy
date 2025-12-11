"use client";

import { forwardRef, InputHTMLAttributes } from "react";
import { cn } from "@/lib/utils/cn";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, helperText, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");

    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={inputId}
            className="block text-sm font-medium text-surface-300 mb-1.5"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={cn(
            "w-full h-10 px-3 bg-surface-900 border border-surface-700 rounded-lg",
            "text-foreground placeholder:text-surface-500",
            "transition-all duration-200",
            "focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500/50",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            error && "border-accent-rose focus:border-accent-rose focus:ring-accent-rose/50",
            className
          )}
          {...props}
        />
        {(error || helperText) && (
          <p
            className={cn(
              "mt-1.5 text-xs",
              error ? "text-accent-rose" : "text-surface-500"
            )}
          >
            {error || helperText}
          </p>
        )}
      </div>
    );
  }
);

Input.displayName = "Input";

