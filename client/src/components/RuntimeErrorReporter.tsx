"use client";

import { useEffect } from "react";
import { initGlobalErrorReporting } from "@/lib/monitoring";

/**
 * Registers window-level error and unhandled-rejection listeners once on
 * mount so browser crashes outside React render trees still reach monitoring.
 * Renders nothing.
 */
export default function RuntimeErrorReporter() {
  useEffect(() => {
    initGlobalErrorReporting();
  }, []);

  return null;
}