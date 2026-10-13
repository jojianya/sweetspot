"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import { AuthError, AuthField, AuthShell, AuthSubmit } from "@/components/auth/AuthChrome";
import { errorMessage } from "@/lib/utils";
import { requestPasswordReset } from "@/lib/api";

export default function ForgotForm() {
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!email.trim()) return setError("Enter your email");
    setSubmitting(true);
    try {
      const res = await requestPasswordReset(email.trim());
      // The server answers identically for unknown emails; surface its
      // message verbatim so we cannot leak existence here either.
      setMessage(res.message);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Navbar backHref="/login" backLabel="Back to login" />
      <AuthShell>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
            Forgot password
          </h1>
          {message ? (
            <p role="status" className="mt-4 text-sm text-zinc-600 dark:text-zinc-400">
              {message}
            </p>
          ) : (
            <form onSubmit={submit} className="mt-6 space-y-4">
              <AuthField
                id="email"
                label="Email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
              />
              <AuthError error={error} />
              <AuthSubmit submitting={submitting} pending="Sending…">
                Send reset link
              </AuthSubmit>
            </form>
          )}
          <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
            <Link href="/login" className="font-semibold text-rose-600 hover:underline">
              Back to login
            </Link>
          </p>
      </AuthShell>
    </>
  );
}
