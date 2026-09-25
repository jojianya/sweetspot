"use client";

import { Component, type ErrorInfo, type ReactNode } from "react";
import { reportError } from "@/lib/monitoring";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

export default class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError(error, {
      kind: "react",
      componentStack: info.componentStack,
    });
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
          <p className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">Something went wrong</p>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">The map editor hit an unexpected error.</p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="rounded-lg bg-rose-600 px-4 py-2 font-medium text-white hover:bg-rose-700"
          >
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}