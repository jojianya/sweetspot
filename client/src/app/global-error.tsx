"use client";

import { useEffect } from "react";
import { Geist, Geist_Mono } from "next/font/google";
import { reportError } from "@/lib/monitoring";
import "./globals.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

// global-error replaces the root layout when it fires, so it must render
// its own <html> and <body>. Errors in the root layout itself (font loading,
// the theme-init script) land here rather than in a child error boundary.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    reportError(error, { kind: "global-boundary" });
  }, [error]);

  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="flex h-dvh flex-col overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950">
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
          <p className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Something went wrong
          </p>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            The application hit an unexpected error. You can try again.
          </p>
          <button
            onClick={reset}
            className="rounded-full bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2 text-sm font-medium text-white shadow-md shadow-rose-600/25 transition-transform hover:shadow-lg hover:shadow-rose-600/30 active:scale-95"
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  );
}
