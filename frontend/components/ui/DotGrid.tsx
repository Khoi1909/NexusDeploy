"use client";

import { useEffect, useRef } from "react";
import { cn } from "@/lib/utils/cn";

interface DotGridProps {
  className?: string;
  dotSize?: number;
  dotColor?: string;
  spacing?: number;
  fadeEdges?: boolean;
}

export function DotGrid({
  className,
  dotSize = 1,
  dotColor = "#3f3f46",
  spacing = 24,
  fadeEdges = true,
}: DotGridProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const resizeObserver = new ResizeObserver(() => {
      const rect = canvas.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;

      canvas.width = rect.width * dpr;
      canvas.height = rect.height * dpr;
      ctx.scale(dpr, dpr);

      draw();
    });

    function draw() {
      if (!ctx || !canvas) return;

      const rect = canvas.getBoundingClientRect();
      ctx.clearRect(0, 0, rect.width, rect.height);

      const cols = Math.ceil(rect.width / spacing) + 1;
      const rows = Math.ceil(rect.height / spacing) + 1;

      for (let i = 0; i < cols; i++) {
        for (let j = 0; j < rows; j++) {
          const x = i * spacing;
          const y = j * spacing;

          // Calculate distance from center for fade effect
          let opacity = 1;
          if (fadeEdges) {
            const centerX = rect.width / 2;
            const centerY = rect.height / 2;
            const maxDist = Math.sqrt(centerX * centerX + centerY * centerY);
            const dist = Math.sqrt(
              Math.pow(x - centerX, 2) + Math.pow(y - centerY, 2)
            );
            opacity = 1 - (dist / maxDist) * 0.7;
          }

          ctx.beginPath();
          ctx.arc(x, y, dotSize, 0, Math.PI * 2);
          ctx.fillStyle = dotColor;
          ctx.globalAlpha = opacity * 0.6;
          ctx.fill();
        }
      }

      ctx.globalAlpha = 1;
    }

    resizeObserver.observe(canvas);

    return () => {
      resizeObserver.disconnect();
    };
  }, [dotSize, dotColor, spacing, fadeEdges]);

  return (
    <canvas
      ref={canvasRef}
      className={cn(
        "absolute inset-0 w-full h-full pointer-events-none",
        className
      )}
      style={{ width: "100%", height: "100%" }}
    />
  );
}

