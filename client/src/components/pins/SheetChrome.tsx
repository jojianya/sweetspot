"use client";

import Link from "next/link";
import { CloseIcon } from "@/components/icons";

/**
 * Shared bottom-sheet header: title plus close button. The five sheet
 * headers (report, add-to-collection, edit) rendered this byte-identically.
 */
export function SheetHeader({ title, onClose }: { title: string; onClose: () => void }) {
  return (
    <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
      <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
        {title}
      </h2>
      <button
        type="button"
        onClick={onClose}
        aria-label="Close"
        className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
      >
        <CloseIcon />
      </button>
    </header>
  );
}

/**
 * Shared signed-out sheet body: sign-in prompt with a login CTA.
 * action completes "Sign in to …", e.g. "report this pin".
 */
export function SheetSignIn({ action }: { action: string }) {
  return (
    <div className="flex-1 overflow-y-auto p-5 text-sm text-zinc-600 dark:text-zinc-400">
      <p className="mb-3">Sign in to {action}.</p>
      <Link
        href="/login"
        className="inline-block rounded-lg bg-rose-600 px-4 py-2 font-medium text-white hover:bg-rose-700"
      >
        Log in
      </Link>
    </div>
  );
}
