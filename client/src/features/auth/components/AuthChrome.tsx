"use client";

import type { InputHTMLAttributes, ReactNode } from "react";

/**
 * Shared chrome for the auth pages (login, register, forgot-password,
 * reset-password). Each page keeps its own fields, validation, submit action
 * and footers; everything byte-identical across the four lives here.
 */
export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 px-4 py-12 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950/30">
      <div className="w-full max-w-sm rounded-2xl border border-zinc-200/60 bg-white/80 p-8 shadow-xl shadow-zinc-900/5 backdrop-blur dark:border-zinc-700/60 dark:bg-zinc-900/80 dark:shadow-zinc-950/40">
        {children}
      </div>
    </div>
  );
}

/** Logo mark plus title and subtitle heading (login and register). */
export function AuthHero({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-6 text-center">
      <span className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-rose-500 to-rose-700 text-white shadow-md shadow-rose-600/30">
        <svg className="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 2a7 7 0 0 0-7 7c0 5.25 7 13 7 13s7-7.75 7-13a7 7 0 0 0-7-7Zm0 9.5A2.5 2.5 0 1 1 12 6.5a2.5 2.5 0 0 1 0 5Z" />
        </svg>
      </span>
      <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
        {title}
      </h1>
      <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">{subtitle}</p>
    </div>
  );
}

/** Labelled text input with the shared auth field styling. */
export function AuthField({
  id,
  label,
  hint,
  ...input
}: {
  id: string;
  label: string;
  hint?: string;
} & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
      >
        {label}
      </label>
      <input
        id={id}
        {...input}
        className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
      />
      {hint && <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">{hint}</p>}
    </div>
  );
}

/** Form-level error line. */
export function AuthError({ error }: { error: string | null }) {
  if (!error) return null;
  return (
    <p role="alert" className="text-sm font-medium text-rose-600 dark:text-rose-400">
      {error}
    </p>
  );
}

/** Full-width gradient submit button with a pending label. */
export function AuthSubmit({
  submitting,
  pending,
  children,
}: {
  submitting: boolean;
  pending: string;
  children: ReactNode;
}) {
  return (
    <button
      type="submit"
      disabled={submitting}
      className="w-full rounded-xl bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2.5 font-medium text-white shadow-md shadow-rose-600/25 transition-all hover:shadow-lg hover:shadow-rose-600/30 active:scale-[0.98] disabled:opacity-60"
    >
      {submitting ? pending : children}
    </button>
  );
}
