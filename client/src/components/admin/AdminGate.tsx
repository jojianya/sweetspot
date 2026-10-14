"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import Navbar from "@/shared/layout/Navbar";
import { useAdminSession } from "@/hooks/useAdminSession";
import { useAuth } from "@/store/auth";

/**
 * Gate for the admin surfaces (reports, roles). The cached role is not a
 * credential, so /me decides whether the screen renders at all: until it
 * answers, authorized is false and the admin tree below is never mounted, so
 * no admin request goes out for a signed-out or unprivileged caller.
 */
export function AdminGate({
  required,
  signedOutTitle,
  deniedTitle,
  deniedCopy,
  children,
}: {
  required: "owner" | "moderator";
  signedOutTitle: string;
  deniedTitle: string;
  deniedCopy: string;
  children: ReactNode;
}) {
  const { user } = useAuth();
  const { authorized, checked } = useAdminSession(required);

  if (!user) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            {signedOutTitle}
          </h1>
          <Link
            href="/login"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </>
    );
  }

  if (!authorized) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            {deniedTitle}
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            {checked ? deniedCopy : "Checking your access…"}
          </p>
          <Link
            href="/"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Back to map
          </Link>
        </div>
      </>
    );
  }

  return <>{children}</>;
}
