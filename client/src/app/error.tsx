"use client";

import { useEffect } from "react";
import Navbar from "@/components/layout/Navbar";
import { reportError } from "@/lib/monitoring";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    reportError(error, { kind: "react-boundary" });
  }, [error]);

  return (
    <div className="flex flex-1 flex-col bg-gradient-to-br from-rose-50 via-white to-orange-50 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950">
      <Navbar />
      <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
        <p className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Something went wrong
        </p>
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          An unexpected error occurred. You can try again.
        </p>
        <button
          onClick={reset}
          className="rounded-full bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2 text-sm font-medium text-white shadow-md shadow-rose-600/25 transition-transform hover:shadow-lg hover:shadow-rose-600/30 active:scale-95"
        >
          Try again
        </button>
      </div>
    </div>
  );
}
