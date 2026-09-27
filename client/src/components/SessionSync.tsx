"use client";

import { useSessionSync } from "@/hooks/useSessionSync";

/**
 * Confines the client boundary for session reconciliation to one leaf
 * component instead of making the whole root layout a client component.
 * Renders nothing.
 */
export default function SessionSync() {
  useSessionSync();
  return null;
}
