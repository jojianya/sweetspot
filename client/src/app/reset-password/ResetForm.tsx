"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import Navbar from "@/shared/layout/Navbar";
import { AuthError, AuthField, AuthShell, AuthSubmit } from "@/components/auth/AuthChrome";
import { checkPassword } from "@/lib/password";
import { confirmPasswordReset } from "@/lib/api/auth";

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
      <AuthShell>
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
              <AuthField
                id="password"
                label="New password"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="At least 8 characters"
              />
              <AuthField
                id="confirm"
                label="Confirm password"
                type="password"
                autoComplete="new-password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
              />
              <AuthError error={error} />
              <AuthSubmit submitting={submitting} pending="Updating…">
                Update password
              </AuthSubmit>
            </form>
          )}
          {!done && (
            <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
              <Link href="/forgot-password" className="font-semibold text-rose-600 hover:underline">
                Request a new link
              </Link>
            </p>
          )}
      </AuthShell>
    </>
  );
}
