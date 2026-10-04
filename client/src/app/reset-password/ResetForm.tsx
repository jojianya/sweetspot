"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import Navbar from "@/components/layout/Navbar";
import { checkPassword } from "@/lib/password";
import { confirmPasswordReset } from "@/lib/api";

const GENERIC_FAILURE = "This link is invalid or has expired. Request a new one below.";

export default function ResetForm() {
  const searchParams = useSearchParams();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Read the token once, then strip it from the address bar immediately.
  // Anything that captures location.href afterwards (error reporter,
  // history, shoulder surfers) never sees it, and with no-referrer plus no
  // external resources on this page it never leaves as a Referer either.
  // The token is only ever sent in a POST body, and no error path includes it.
  // The initial-state read (not the effect) owns the token: search-param
  // objects are not guaranteed stable across renders, so re-reading later
  // could observe the already-stripped URL and wipe it.
  const [token] = useState<string | null>(() => {
    const t = searchParams.get("token");
    return t && t.length > 0 ? t : null;
  });
  useEffect(() => {
    window.history.replaceState(null, "", window.location.pathname);
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!token) return setError(GENERIC_FAILURE);
    const check = checkPassword(password);
    if (!check.ok) return setError(check.message);
    if (password !== confirm) return setError("Passwords do not match");
    setSubmitting(true);
    try {
      await confirmPasswordReset(token, password);
      setDone(true);
    } catch {
      // Deliberately generic and token-free: unknown, expired, used and
      // superseded links are indistinguishable by design.
      setError(GENERIC_FAILURE);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Navbar backHref="/login" backLabel="Back to login" />
      <div className="flex flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 px-4 py-12 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950/30">
        <div className="w-full max-w-sm rounded-2xl border border-zinc-200/60 bg-white/80 p-8 shadow-xl shadow-zinc-900/5 backdrop-blur dark:border-zinc-700/60 dark:bg-zinc-900/80 dark:shadow-zinc-950/40">
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
            Set a new password
          </h1>
          {done ? (
            <>
              <p role="status" className="mt-4 text-sm text-zinc-600 dark:text-zinc-400">
                Password updated. All other sessions were signed out.
              </p>
              <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
                <Link href="/login" className="font-semibold text-rose-600 hover:underline">
                  Log in with your new password
                </Link>
              </p>
            </>
          ) : (
            <form onSubmit={submit} className="mt-6 space-y-4">
              <div>
                <label
                  htmlFor="password"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  New password
                </label>
                <input
                  id="password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
                  placeholder="At least 8 characters"
                />
              </div>
              <div>
                <label
                  htmlFor="confirm"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Confirm password
                </label>
                <input
                  id="confirm"
                  type="password"
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
                />
              </div>
              {error && (
                <p role="alert" className="text-sm font-medium text-rose-600 dark:text-rose-400">
                  {error}
                </p>
              )}
              <button
                type="submit"
                disabled={submitting}
                className="w-full rounded-xl bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2.5 font-medium text-white shadow-md shadow-rose-600/25 transition-all hover:shadow-lg hover:shadow-rose-600/30 active:scale-[0.98] disabled:opacity-60"
              >
                {submitting ? "Updating…" : "Update password"}
              </button>
            </form>
          )}
          {!done && (
            <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
              <Link href="/forgot-password" className="font-semibold text-rose-600 hover:underline">
                Request a new link
              </Link>
            </p>
          )}
        </div>
      </div>
    </>
  );
}
