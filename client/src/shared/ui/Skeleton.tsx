"use client";

import { forwardRef } from "react";

/**
 * Minimal skeleton placeholder. Uses the app's zinc color scale and
 * respects prefers-reduced-motion via motion-safe:animate-pulse.
 * Decorative only — aria-hidden="true".
 */
export const Skeleton = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className = "", style, ...props }, ref) => (
    <div
      ref={ref}
      aria-hidden="true"
      className={`bg-zinc-200 dark:bg-zinc-700 motion-safe:animate-pulse ${className}`}
      style={style}
      {...props}
    />
  )
);

Skeleton.displayName = "Skeleton";

/**
 * Wrapper for a loading region. Adds aria-busy="true", role="status",
 * and an sr-only "Loading…" label. Use once per loading region.
 */
export function SkeletonRegion({
  children,
  label = "Loading…",
}: {
  children: React.ReactNode;
  label?: string;
}) {
  return (
    <div role="status" aria-busy="true" aria-label={label}>
      <span className="sr-only">{label}</span>
      {children}
    </div>
  );
}