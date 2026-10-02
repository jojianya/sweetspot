"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
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
      <div className="flex flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 px-4 py-12 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950/30">
        <div className="w-full max-w-sm rounded-2xl border border-zinc-200/60 bg-white/80 p-8 shadow-xl shadow-zinc-900/5 backdrop-blur dark:border-zinc-700/60 dark:bg-zinc-900/80 dark:shadow-zinc-950/40">
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
            Forgot password
          </h1>
          {message ? (
            <p role="status" className="mt-4 text-sm text-zinc-600 dark:text-zinc-400">
              {message}
            </p>
          ) : (
            <form onSubmit={submit} className="mt-6 space-y-4">
              <div>
                <label
                  htmlFor="email"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
                  placeholder="you@example.com"
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
                {submitting ? "Sending…" : "Send reset link"}
              </button>
            </form>
          )}
          <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
            <Link href="/login" className="font-semibold text-rose-600 hover:underline">
              Back to login
            </Link>
          </p>
        </div>
      </div>
    </>
  );
}
